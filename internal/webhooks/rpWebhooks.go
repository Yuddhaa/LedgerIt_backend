package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5/pgtype"
	razorpayUtils "github.com/razorpay/razorpay-go/utils"
)

type webhookPayload struct {
	Event   string         `json:"event"`
	Payload map[string]any `json:"payload"`
}

type SubscriptionEntity struct {
	ID        string         `json:"id"`
	Status    string         `json:"status"`
	PlanID    string         `json:"plan_id"`
	StartAt   int64          `json:"start_at"`
	ChargeAt  int64          `json:"charge_at"`
	PaidCount int            `json:"paid_count"`
	Notes     map[string]any `json:"notes"`
}

func (h *Handler) RazorpayWebhooks(w http.ResponseWriter, r *http.Request) {
	// 1. Read Body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		helpers.LogError("RazorpayHandler", "bad request", "err", err.Error())
		return
	}
	bodyStr := string(bodyBytes)

	// 2. Verify Signature
	signature := r.Header.Get("X-Razorpay-Signature")
	if !razorpayUtils.VerifyWebhookSignature(bodyStr, signature, h.razorpayWebhookSecret) {
		helpers.LogError("RazorpayHandler", "invalid signature", "sig", signature)
		helpers.RespondWithError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	// 3. Parse JSON
	var payload webhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		helpers.LogError("RazorpayHandler", "failed to parse json", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	helpers.LogInfo("RazorpayHandler", "Event Received", "event", payload.Event)

	// 4. Handle Events
	var handlerErr error
	ctx := r.Context()

	switch payload.Event {
	// case "subscription.authenticated":
	// 	handlerErr = h.handleSubscriptionAuthenticated(ctx, payload.Payload)
	case "subscription.charged":
		handlerErr = h.handleSubscriptionCharged(ctx, payload.Payload)
	case "subscription.cancelled":
		handlerErr = h.handleSubscriptionCancelled(ctx, payload.Payload)
	case "subscription.paused":
		handlerErr = h.handleSubscriptionPaused(ctx, payload.Payload)
	case "subscription.resumed":
		handlerErr = h.handleSubscriptionResumed(ctx, payload.Payload)
	case "subscription.pending":
		handlerErr = h.handleSubscriptionPending(ctx, payload.Payload)
	case "subscription.halted":
		handlerErr = h.handleSubscriptionHalted(ctx, payload.Payload)
	case "order.paid":
		handlerErr = h.handleOrderPaid(ctx, payload.Payload)
	}
	if handlerErr != nil {
		// CRITICAL FIX: Return 500 to trigger Razorpay Retry
		// We trust that our handlers only return errors for "Retryable" issues (DB connection, Lock timeout, etc.)
		helpers.LogError("RazorpayHandler", "Transient Error - Requesting Retry", "event", payload.Event, "err", handlerErr.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Success
	w.WriteHeader(http.StatusOK)
}

// ********************************************************************************************************************
// ********************************************************************************************************************
// -- helper functions --
// ********************************************************************************************************************
// ********************************************************************************************************************

// Helper to extract common data safely
func extractSubscriptionData(payload map[string]any) (string, map[string]any, error) {
	// Safety checks for nil maps should be added if payload is unpredictable,
	// but Razorpay structure is consistent for these events.
	subEntity, ok := payload["subscription"].(map[string]any)["entity"].(map[string]any)
	if !ok {
		return "", map[string]any{}, fmt.Errorf("cannot convert payload subscription or entity")
	}
	rzpSubID, ok := subEntity["id"].(string)
	if !ok {
		return "", map[string]any{}, fmt.Errorf("cannot convert payload id")
	}
	notes, ok := subEntity["notes"].(map[string]any)
	if !ok {
		return "", map[string]any{}, fmt.Errorf("cannot convert notes")
	}
	return rzpSubID, notes, nil
}

// Helper to handle simple state transitions (Paused, Resumed, Pending, Halted)
// It includes the "Zombie Guard" to prevent resurrecting cancelled subscriptions.
func (h *Handler) updateSubscriptionStateSafe(ctx context.Context, payload map[string]any, targetStatus db.SubscriptionsStatus, eventName string) error {
	rzpSubID, _, err := extractSubscriptionData(payload)
	if err != nil {
		helpers.LogError("updateSubscriptionStateSafe", "err in extractSubscriptionData", "err", err.Error())
		return err
	}

	// 1. Fetch Current State
	sub, err := h.db.GetSubscriptionByRazorpayID(ctx, rzpSubID)
	if err != nil {
		// Return error to trigger Razorpay retry (in case of DB glitches)
		return err
	}

	// 2. Zombie Guard
	if sub.Status == db.SubscriptionsStatusCanceled {
		helpers.LogInfo("Webhook", fmt.Sprintf("Ignoring '%s' event - Subscription is Cancelled", eventName), "id", rzpSubID)
		return nil // Return nil to swallow the event (success)
	}

	// 3. Update Status
	helpers.LogInfo("Webhook", fmt.Sprintf("Processing '%s' -> Setting status to '%s'", eventName, targetStatus), "id", rzpSubID)
	return h.db.UpdateSubscriptionStatusRaw(ctx, db.UpdateSubscriptionStatusRawParams{
		RazorpaySubscriptionID: rzpSubID,
		Status:                 targetStatus,
	})
}

// ********************************************************************************************************************
// ********************************************************************************************************************
// handlers for each event
// ********************************************************************************************************************
// ********************************************************************************************************************

func (h *Handler) handleSubscriptionCharged(ctx context.Context, payload map[string]any) error {
	// ****************************************************************************************************************
	// extarct the subscription data
	// ****************************************************************************************************************
	rzpSubID, notes, err := extractSubscriptionData(payload)
	if err != nil {
		helpers.LogError("handleSubscriptionCharged", "error in extractSubscriptionData", "err", err.Error())
		return err
	}

	// ****************************************************************************************************************
	// start a db Transaction
	// ****************************************************************************************************************
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := h.db.WithTx(tx)

	// ****************************************************************************************************************
	// extarct some more requrired data
	// ****************************************************************************************************************
	paymentEntity, ok := payload["payment"].(map[string]any)["entity"].(map[string]any)
	if !ok {
		return fmt.Errorf("cannot convert payload payments or entity")
	}
	paymentID, ok := paymentEntity["id"].(string)
	if !ok {
		return fmt.Errorf("cannot convert payload id")
	}
	temp_amount_float64, ok := paymentEntity["amount"].(float64)
	if !ok {
		return fmt.Errorf("cannot convert payload amount")
	}
	amount := int64(temp_amount_float64)

	subEntity, ok := payload["subscription"].(map[string]any)["entity"].(map[string]any)
	if !ok {
		return fmt.Errorf("cannot convert payload subscription or entity")
	}
	temp_startAt_float64, ok := subEntity["current_start"].(float64)
	if !ok {
		return fmt.Errorf("cannot convert payload start at")
	}
	currentStart := time.Unix(int64(temp_startAt_float64), 0)
	temp_endAt_float64, ok := subEntity["current_end"].(float64)
	if !ok {
		return fmt.Errorf("cannot convert payload endAT")
	}
	currentEnd := time.Unix(int64(temp_endAt_float64), 0)

	// CRITICAL FIX: Get paid_count to detect renewals
	paidCount := int(subEntity["paid_count"].(float64))

	// 3. Get Subscription
	subRow, err := qtx.GetSubscriptionByRazorpayID(ctx, rzpSubID)
	if err != nil {
		helpers.LogError("Webhook", "Subscription lookup failed", "id", rzpSubID, "err", err.Error())
		return err
	}

	// ZOMBIE GUARD
	// If the subscription is already cancelled, a late 'charged' event should not reactivate it.
	if subRow.Status == db.SubscriptionsStatusCanceled {
		helpers.LogInfo("Webhook", "Ignored 'charged' event for Cancelled subscription", "id", rzpSubID, "pay_id", paymentID)

		// Optional: You COULD create the invoice here just for record-keeping if you wanted.
		// For now, we simply return nil to consume the event and prevent resurrection.
		return nil
	}

	// ****************************************************************************************************************
	// create invoice
	// ****************************************************************************************************************
	_, err = qtx.CreateInvoice(ctx, db.CreateInvoiceParams{
		SubscriptionID:    subRow.ID,
		BusinessID:        subRow.BusinessID,
		RazorpayPaymentID: paymentID,
		AmountPaid:        amount,
		Currency:          pgtype.Text{String: "INR", Valid: true},
		Status:            db.SubscriptionInvoicesStatusPaid,
	})
	if err != nil {
		if helpers.IsUniqueViolation(err) {
			helpers.LogInfo("Webhook", "Duplicate payment webhook ignored")
		} else {
			helpers.LogError("Webhook", "Invoice creation failed", "err", err.Error())
		}
	}

	// ****************************************************************************************************************
	//  update the business and subscription table
	// ****************************************************************************************************************
	finalStatus := db.SubscriptionsStatusActive
	statusReason := "payment received"

	// Check if this is a Trial
	isTrialNote := false
	if val, ok := notes["is_trialing"].(string); ok && val == "1" {
		isTrialNote = true
	}
	if val, ok := notes["is_trialing"].(bool); ok && val {
		isTrialNote = true
	}

	isTrial := false
	// THE FIX: Only set to 'Trialing' if it is the VERY FIRST payment (Auth Fee)
	// If paid_count > 1, it is a renewal, so it must be Active (Paid).
	if isTrialNote && paidCount <= 1 {
		finalStatus = db.SubscriptionsStatusTrialing
		statusReason = "trial auth fee"
		isTrial = true
	}

	// 6. Update Subscription & Business
	err = qtx.UpdateSubscriptionAndBusiness(ctx, db.UpdateSubscriptionAndBusinessParams{
		RazorpaySubscriptionID: rzpSubID,
		Status:                 finalStatus,
		CurrentPeriodStart:     pgtype.Timestamptz{Time: currentStart, Valid: true},
		CurrentPeriodEnd:       pgtype.Timestamptz{Time: currentEnd, Valid: true},
		IsTrialUsed:            pgtype.Bool{Bool: isTrial, Valid: true},
	})
	if err != nil {
		helpers.LogInfo("handleSubscriptionCharged", "err in UpdateSubscriptionAndBusiness", "err", err.Error())
		return err
	}
	helpers.LogInfo("Webhook", "Subscription Activated/Renewed", "sub_id", rzpSubID, "status", finalStatus, "reason", statusReason)

	// ****************************************************************************************************************
	// cancel the old sub
	// ****************************************************************************************************************
	if subRow.Status != db.SubscriptionsStatusActive {
		if typeVal, ok := notes["type"].(string); ok && (typeVal == "update" || typeVal == "upgrade") {
			oldSubID, _ := notes["old_sub_razorpay_id"].(string)
			isImmediate, _ := notes["is_immediate"].(string)

			if oldSubID != "" {
				cancelAtEnd := 1 // Default: Cycle End
				if isImmediate == "1" {
					cancelAtEnd = 0
				} // Immediate

				// Fire and Forget. We trust the Webhook System to send us 'subscription.cancelled'
				_, rpErr := h.rp_client.Subscription.Cancel(oldSubID, map[string]any{"cancel_at_cycle_end": cancelAtEnd}, nil)
				if rpErr != nil {
					helpers.LogError("Webhook", "Razorpay Cancel Call Failed", "old", oldSubID, "err", rpErr.Error())
				}
				helpers.LogInfo("Webhook", "Triggered Cancellation", "old_id", oldSubID)
			}
		}
	}

	// 8. Commit
	return tx.Commit(ctx)
}

func (h *Handler) handleSubscriptionCancelled(ctx context.Context, payload map[string]any) error {
	rzpSubID, _, err := extractSubscriptionData(payload)
	if err != nil {
		helpers.LogError("handleSubscriptionCancelled", "err in extractSubscriptionData", "err", err.Error())
		return err
	}

	helpers.LogInfo("Webhook", "Processing Cancellation", "id", rzpSubID)

	// atomic update that safely unlinks the subscription
	err = h.db.CancelSubscription(ctx, rzpSubID)
	if err != nil {
		// If rows affected is 0 (because it was already cancelled or not found),
		// sqlc might return nil or ErrNoRows depending on config.
		// Usually we just log errors here.
		helpers.LogError("Webhook", "Cancellation Failed", "id", rzpSubID, "err", err.Error())
		return err
	}

	helpers.LogInfo("Webhook", "Subscription Cancelled Successfully", "id", rzpSubID)
	return nil
}

func (h *Handler) handleSubscriptionPaused(ctx context.Context, payload map[string]any) error {
	return h.updateSubscriptionStateSafe(ctx, payload, db.SubscriptionsStatusPaused, "subscription.paused")
}

func (h *Handler) handleSubscriptionResumed(ctx context.Context, payload map[string]any) error {
	// 'resumed' maps to 'active'
	return h.updateSubscriptionStateSafe(ctx, payload, db.SubscriptionsStatusActive, "subscription.resumed")
}

func (h *Handler) handleSubscriptionPending(ctx context.Context, payload map[string]any) error {
	return h.updateSubscriptionStateSafe(ctx, payload, db.SubscriptionsStatusPending, "subscription.pending")
}

func (h *Handler) handleSubscriptionHalted(ctx context.Context, payload map[string]any) error {
	// 'halted' maps to 'past_due'
	return h.updateSubscriptionStateSafe(ctx, payload, db.SubscriptionsStatusPastDue, "subscription.halted")
}

func (h *Handler) handleOrderPaid(ctx context.Context, payload map[string]any) error {
	// ****************************************************************************************************************
	// 1. Extract Order & Payment Data
	// ****************************************************************************************************************
	// Safely extract nested maps. In webhooks, these are usually map[string]interface{}
	paymentEntity, ok := payload["payment"].(map[string]any)["entity"].(map[string]any)
	if !ok {
		return fmt.Errorf("handleOrderPaid: cannot extract payment entity")
	}
	orderEntity, ok := payload["order"].(map[string]any)["entity"].(map[string]any)
	if !ok {
		return fmt.Errorf("handleOrderPaid: cannot extract order entity")
	}

	// Extract IDs
	paymentID, _ := paymentEntity["id"].(string)
	orderID, _ := orderEntity["id"].(string)

	// Razorpay sends amount in Paisa (float64 in JSON decoding usually)
	tempAmount, _ := paymentEntity["amount"].(float64)
	amountPaid := int64(tempAmount)

	// Extract Notes (Crucial for linking to business/user)
	notes, ok := orderEntity["notes"].(map[string]any)
	if !ok {
		helpers.LogInfo("Webhook", "handleOrderPaid ignored: no notes found", "order_id", orderID)
		return nil // Ignore irrelevant orders
	}
	// ****************************************************************************************************************
	// 3. Start DB Transaction
	// ****************************************************************************************************************
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := h.db.WithTx(tx)

	// ****************************************************************************************************************
	// 4. Get The Pending Subscription Row
	// ****************************************************************************************************************
	// In createOrder, we stored the 'order_id' in the 'razorpay_subscription_id' column.
	// We fetch it to get the internal Primary Key (ID).
	subRow, err := qtx.GetSubscriptionByRazorpayID(ctx, orderID)
	if err != nil {
		helpers.LogError("Webhook", "handleOrderPaid: Subscription lookup failed", "order_id", orderID, "err", err.Error())
		return err
	}

	// ZOMBIE GUARD: If already Active, do not re-process
	if subRow.Status == db.SubscriptionsStatusActive {
		helpers.LogInfo("Webhook", "Order already processed", "order_id", orderID)
		return nil
	}

	// ****************************************************************************************************************
	// 5. Create Invoice (Record Keeping)
	// ****************************************************************************************************************
	_, err = qtx.CreateInvoice(ctx, db.CreateInvoiceParams{
		SubscriptionID:    subRow.ID,
		BusinessID:        subRow.BusinessID,
		RazorpayPaymentID: paymentID,
		AmountPaid:        amountPaid,
		Currency:          pgtype.Text{String: "INR", Valid: true},
		Status:            db.SubscriptionInvoicesStatusPaid,
	})
	if err != nil {
		if helpers.IsUniqueViolation(err) {
			helpers.LogInfo("Webhook", "Duplicate invoice creation ignored", "pay_id", paymentID)
		} else {
			helpers.LogError("Webhook", "Invoice creation failed", "err", err.Error())
			return err
		}
	}

	// ****************************************************************************************************************
	// 6. Update Subscription & Business (Atomic Update)
	// ****************************************************************************************************************
	// Calculate Dates
	startDate := time.Now()
	expiryDate := time.Now().AddDate(100, 0, 0) // 100 Years

	// We use orderID as RazorpaySubscriptionID because that is how we stored it in createOrder.
	// This query will:
	// 1. Update the 'subscriptions' row to Active and set dates.
	// 2. Update the 'businesses' table to point to this subscription and sync the status/plan.
	err = qtx.UpdateSubscriptionAndBusiness(ctx, db.UpdateSubscriptionAndBusinessParams{
		RazorpaySubscriptionID: orderID,
		Status:                 db.SubscriptionsStatusActive,
		CurrentPeriodStart:     pgtype.Timestamptz{Time: startDate, Valid: true},
		CurrentPeriodEnd:       pgtype.Timestamptz{Time: expiryDate, Valid: true},

		// Passing Valid: false (NULL) results in "is_trial_used OR false" in your SQL.
		// This preserves the existing value as requested.
		IsTrialUsed: pgtype.Bool{Valid: false},
	})
	if err != nil {
		helpers.LogError("Webhook", "handleOrderPaid: Failed to update subscription/business", "err", err.Error())
		return err
	}

	helpers.LogInfo("Webhook", "One-Time Order Processed. Plan Activated.", "order_id", orderID)

	// ****************************************************************************************************************
	// 7. Cancel Old Subscription (Upgrade Path)
	// ****************************************************************************************************************
	// Check notes for "old_sub_razorpay_id" passed from UpdateHandler
	// We cancel it IMMEDIATELY because the user has paid for the new Lifetime plan.
	if typeVal, ok := notes["type"].(string); ok && typeVal == "update" {
		oldSubID, _ := notes["old_sub_razorpay_id"].(string)

		if oldSubID != "" {
			// Using helper function logic here
			// cancel_at_cycle_end = 0 (Immediate)
			_, rpErr := h.rp_client.Subscription.Cancel(oldSubID, map[string]any{"cancel_at_cycle_end": 0}, nil)

			if rpErr != nil {
				// Ignore if already cancelled
				if !strings.Contains(rpErr.Error(), "BAD_REQUEST_ERROR") && !strings.Contains(rpErr.Error(), "not in active state") {
					helpers.LogError("Webhook", "Failed to cancel old recurring sub", "old_id", oldSubID, "err", rpErr.Error())
				}
			} else {
				helpers.LogInfo("Webhook", "Old recurring subscription cancelled", "old_id", oldSubID)
			}
		}
	}

	// 8. Commit
	return tx.Commit(ctx)
}
