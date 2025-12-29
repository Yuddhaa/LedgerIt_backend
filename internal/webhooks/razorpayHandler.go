package webhooks

import (
	"encoding/json"
	"io"
	"net/http"

	"LedgerIt/internal/helpers"

	razorpayUtils "github.com/razorpay/razorpay-go/utils"
)

type webhookPayload struct {
	Event   string         `json:"event"`
	Payload map[string]any `json:"payload"`
}

func (h *Handler) RazorpayHandler(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		helpers.LogError("RazorpayHandler", "bad request", "err", err.Error())
		return
	}
	bodyStr := string(bodyBytes)
	signature := r.Header.Get("X-Razorpay-Signature")

	if !razorpayUtils.VerifyWebhookSignature(bodyStr, signature, h.razorpayWebhookSecret) {
		helpers.LogError("RazorpayHandler", "invalid signature", "sig", signature)
		helpers.RespondWithError(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	var payload webhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		helpers.LogError("RazorpayHandler", "failed to parse json", "err", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	helpers.LogInfo("RazorpayHandler", "body str", "events", payload.Event)
	// ****************************************************************************************************************
	// based on the events do the respective updates.
	// ****************************************************************************************************************
}
