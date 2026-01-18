package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
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
	// 1. Extract Data
	// ****************************************************************************************************************
	rzpSubID, notes, err := extractSubscriptionData(payload)
	if err != nil {
		helpers.LogError("handleSubscriptionCharged", "error in extractSubscriptionData", "err", err.Error())
		return err
	}

	// ****************************************************************************************************************
	// 2. Start Transaction
	// ****************************************************************************************************************
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := h.db.WithTx(tx)

	// ****************************************************************************************************************
	// 3. Extract Payment Details
	// ****************************************************************************************************************
	paymentEntity, ok := payload["payment"].(map[string]any)["entity"].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid payment entity")
	}
	paymentID, _ := paymentEntity["id"].(string)
	tempAmount, _ := paymentEntity["amount"].(float64)
	amount := int64(tempAmount)

	subEntity, ok := payload["subscription"].(map[string]any)["entity"].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid subscription entity")
	}

	// Parse Dates
	startFloat, _ := subEntity["current_start"].(float64)
	endFloat, _ := subEntity["current_end"].(float64)
	currentStart := time.Unix(int64(startFloat), 0)
	currentEnd := time.Unix(int64(endFloat), 0)

	// Paid Count (Crucial for Trial vs Renewal)
	paidCount := int(subEntity["paid_count"].(float64))

	// ****************************************************************************************************************
	// 4. Get Subscription & Zombie Guard
	// ****************************************************************************************************************
	subRow, err := qtx.GetSubscriptionByRazorpayID(ctx, rzpSubID)
	if err != nil {
		helpers.LogError("Webhook", "Subscription lookup failed", "id", rzpSubID)
		return err
	}

	// If already cancelled, do not reactivate.
	if subRow.Status == db.SubscriptionsStatusCanceled {
		helpers.LogInfo("Webhook", "Zombie Guard: Ignored event for cancelled sub", "id", rzpSubID)
		return nil
	}

	// ****************************************************************************************************************
	// 5. Create Invoice
	// ****************************************************************************************************************
	// REQUIRED: SQL must use "ON CONFLICT (razorpay_payment_id) DO NOTHING"
	_, err = qtx.CreateInvoice(ctx, db.CreateInvoiceParams{
		SubscriptionID:    subRow.ID,
		BusinessID:        subRow.BusinessID,
		RazorpayPaymentID: paymentID,
		AmountPaid:        amount,
		Currency:          pgtype.Text{String: "INR", Valid: true},
		Status:            db.SubscriptionInvoicesStatusPaid,
	})
	if err != nil {
		// pgx.ErrNoRows means "DO NOTHING" triggered (Duplicate). This is Success.
		// If it's a REAL error (connection lost), return it.
		if !errors.Is(err, pgx.ErrNoRows) {
			helpers.LogError("Webhook", "Invoice creation failed", "err", err.Error())
			return err
		}
		// Log but continue. Transaction is ALIVE.
		helpers.LogInfo("Webhook", "Invoice exists (Idempotent)", "pay_id", paymentID)
	}

	// ****************************************************************************************************************
	// 6. Update Subscription & Business (Atomic)
	// ****************************************************************************************************************
	finalStatus := db.SubscriptionsStatusActive
	statusReason := "payment received"

	// Trial Logic: If Note says 'trialing' AND this is the 1st payment (Auth Fee)
	isTrialNote := false
	if val, ok := notes["is_trialing"].(string); ok && val == "1" {
		isTrialNote = true
	}
	if val, ok := notes["is_trialing"].(bool); ok && val {
		isTrialNote = true
	}

	isTrial := false
	if isTrialNote && paidCount <= 1 {
		finalStatus = db.SubscriptionsStatusTrialing
		statusReason = "trial auth fee"
		isTrial = true
	}

	// Offer Code Extraction
	offerCode := ""
	if val, ok := notes["offer_code"].(string); ok {
		offerCode = val
	}
	helpers.LogInfo("handleSubscriptionCharged", "offerCode", "offerCode", offerCode)

	err = qtx.UpdateSubscriptionAndBusiness(ctx, db.UpdateSubscriptionAndBusinessParams{
		RazorpaySubscriptionID: rzpSubID,
		Status:                 finalStatus,
		CurrentPeriodStart:     pgtype.Timestamptz{Time: currentStart, Valid: true},
		CurrentPeriodEnd:       pgtype.Timestamptz{Time: currentEnd, Valid: true},
		IsTrialUsed:            pgtype.Bool{Bool: isTrial, Valid: true},

		// FIX: Use pgtype.Text.
		// Valid: offerCode != "" ensures we send NULL if empty, preserving the DB value via COALESCE.
		OfferCode: pgtype.Text{String: offerCode, Valid: offerCode != ""},
	})
	if err != nil {
		helpers.LogError("Webhook", "UpdateSubscriptionAndBusiness failed", "err", err.Error())
		return err
	}
	helpers.LogInfo("Webhook", "Subscription Updated", "status", finalStatus, "reason", statusReason)

	// ****************************************************************************************************************
	// 7. Marketer Commission
	// ****************************************************************************************************************
	if amount > 0 && subRow.MarketerID.Valid {
		marketer, err := qtx.GetMarketerById(ctx, subRow.MarketerID)
		if err == nil {
			// Integer Math: (AmountPaise * Percent) / 100
			commissionAmount := (amount * int64(marketer.CommissionPercent)) / 100

			if commissionAmount > 0 {
				err = qtx.AddMarketerCommission(ctx, db.AddMarketerCommissionParams{
					ID:     subRow.MarketerID,
					Amount: commissionAmount, // Amount to ADD
				})
				if err != nil {
					helpers.LogError("Webhook", "AddCommission failed", "err", err.Error())
					// Don't fail the request, just log.
				} else {
					helpers.LogInfo("Webhook", "Commission Added", "amt", commissionAmount)
				}
			}
		}
	}

	// ****************************************************************************************************************
	// 8. Cancel Old Subscription (Upgrade Path)
	// ****************************************************************************************************************
	// We check the Note Type directly.
	// Idempotency: Calling Cancel on an already cancelled sub in Razorpay returns an error we can ignore or handle.
	if typeVal, ok := notes["type"].(string); ok && (typeVal == "update" || typeVal == "upgrade") {
		oldSubID, _ := notes["old_sub_razorpay_id"].(string)

		// Only cancel if we have an ID and it's NOT the current one
		if oldSubID != "" && oldSubID != rzpSubID {
			isImmediate, _ := notes["is_immediate"].(string)
			cancelAtEnd := 1
			if isImmediate == "1" {
				cancelAtEnd = 0
			}

			_, rpErr := h.rp_client.Subscription.Cancel(oldSubID, map[string]any{"cancel_at_cycle_end": cancelAtEnd}, nil)
			if rpErr != nil {
				// Ignore benign errors (already cancelled)
				if !strings.Contains(rpErr.Error(), "BAD_REQUEST_ERROR") {
					helpers.LogError("Webhook", "Old Sub Cancel failed", "id", oldSubID, "err", rpErr.Error())
				}
			} else {
				helpers.LogInfo("Webhook", "Old Sub Cancelled", "id", oldSubID)
			}
		}
	}

	// ****************************************************************************************************************
	// 9. Commit
	// ****************************************************************************************************************
	return tx.Commit(ctx)
}

func (h *Handler) handleSubscriptionCancelled(ctx context.Context, payload map[string]any) error {
	rzpSubID, _, err := extractSubscriptionData(payload)
	if err != nil {
		helpers.LogError("handleSubscriptionCancelled", "err in extractSubscriptionData", "err", err.Error())
		return err
	}

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

	// Razorpay sends amount in Paisa
	tempAmount, _ := paymentEntity["amount"].(float64)
	amountPaid := int64(tempAmount)

	// Extract Notes (Crucial for linking to business/user/offer)
	notes, ok := orderEntity["notes"].(map[string]any)
	if !ok {
		helpers.LogInfo("Webhook", "handleOrderPaid ignored: no notes found", "order_id", orderID)
		return nil // Ignore irrelevant orders
	}

	// ****************************************************************************************************************
	// 2. Start DB Transaction
	// ****************************************************************************************************************
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := h.db.WithTx(tx)

	// ****************************************************************************************************************
	// 3. Get The Pending Subscription Row
	// ****************************************************************************************************************
	// In createOrder, we stored 'order_id' in 'razorpay_subscription_id' column.
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
	// 4. Create Invoice (Idempotent Fix)
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
		// FIX: Use ErrNoRows (ON CONFLICT DO NOTHING) instead of catching Unique Violation
		if !errors.Is(err, pgx.ErrNoRows) {
			helpers.LogError("Webhook", "Invoice creation failed", "err", err.Error())
			return err
		}
		// Success (Idempotent)
		helpers.LogInfo("Webhook", "Duplicate invoice creation ignored (Idempotent)", "pay_id", paymentID)
	}

	// ****************************************************************************************************************
	// 5. Update Subscription & Business (With Offer Code)
	// ****************************************************************************************************************
	startDate := time.Now()
	expiryDate := time.Now().AddDate(100, 0, 0) // 100 Years

	// Extract Offer Code
	offerCode := ""
	if val, ok := notes["offer_code"].(string); ok {
		offerCode = val
	}

	err = qtx.UpdateSubscriptionAndBusiness(ctx, db.UpdateSubscriptionAndBusinessParams{
		RazorpaySubscriptionID: orderID,
		Status:                 db.SubscriptionsStatusActive,
		CurrentPeriodStart:     pgtype.Timestamptz{Time: startDate, Valid: true},
		CurrentPeriodEnd:       pgtype.Timestamptz{Time: expiryDate, Valid: true},
		IsTrialUsed:            pgtype.Bool{Valid: false}, // Keep existing value

		// FIX: Pass Offer Code properly
		OfferCode: pgtype.Text{String: offerCode, Valid: offerCode != ""},
	})
	if err != nil {
		helpers.LogError("Webhook", "handleOrderPaid: Failed to update subscription/business", "err", err.Error())
		return err
	}

	helpers.LogInfo("Webhook", "One-Time Order Processed. Plan Activated.", "order_id", orderID)

	// ****************************************************************************************************************
	// 6. HANDLE MARKETER COMMISSION
	// ****************************************************************************************************************
	if amountPaid > 0 && subRow.MarketerID.Valid {
		marketer, err := qtx.GetMarketerById(ctx, subRow.MarketerID)

		if err != nil {
			helpers.LogError("Webhook", "Failed to fetch marketer for commission", "m_id", subRow.MarketerID, "err", err.Error())
		} else {
			commissionAmount := (amountPaid * int64(marketer.CommissionPercent)) / 100

			if commissionAmount > 0 {
				err = qtx.AddMarketerCommission(ctx, db.AddMarketerCommissionParams{
					ID:     subRow.MarketerID,
					Amount: commissionAmount,
				})
				if err != nil {
					helpers.LogError("Webhook", "Failed to add commission", "m_id", subRow.MarketerID, "err", err.Error())
				} else {
					helpers.LogInfo("Webhook", "Commission Added", "m_id", subRow.MarketerID, "amount", commissionAmount)
				}
			}
		}
	}

	// ****************************************************************************************************************
	// 7. Cancel Old Subscription (Upgrade Path)
	// ****************************************************************************************************************
	if typeVal, ok := notes["type"].(string); ok && typeVal == "update" {
		oldSubID, _ := notes["old_sub_razorpay_id"].(string)

		if oldSubID != "" {
			_, rpErr := h.rp_client.Subscription.Cancel(oldSubID, map[string]any{"cancel_at_cycle_end": 0}, nil)
			if rpErr != nil {
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
