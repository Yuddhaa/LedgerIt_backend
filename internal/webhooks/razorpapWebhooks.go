package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

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

// -- helper functions --

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
