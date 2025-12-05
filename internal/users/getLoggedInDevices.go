package users

import (
	"net/http"
	"time"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mssola/user_agent" // Library to parse raw strings into "Chrome", "iPhone", etc.
)

type DeviceResponse struct {
	TokenID      pgtype.UUID `json:"token_id"`    // Useful for "Log out this device"
	DeviceName   string      `json:"device_name"` // e.g. "iPhone", "Macintosh"
	Browser      string      `json:"browser"`     // e.g. "Safari 14.0"
	OS           string      `json:"os"`          // e.g. "iOS 14.2"
	RawUserAgent string      `json:"raw_user_agent"`
	LoginTime    time.Time   `json:"login_time"`
	ExpiresAt    time.Time   `json:"expires_at"`
	IsCurrent    bool        `json:"is_current"` // Helper to highlight the current session
}

func (h *Handler) GetLoggedInDevices(w http.ResponseWriter, r *http.Request) {
	userId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}

	// 1. Fetch from DB
	tokens, err := h.db.ListUserDevices(r.Context(), userId)
	if err != nil {
		helpers.LogError("GetUserDevicesHandler", "db error fetching devices", "err", err)
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// 2. Parse Data
	currentUA := r.UserAgent() // Get current request's UA to compare

	var devices []DeviceResponse

	for _, t := range tokens {
		var uaString string
		if t.DeviceInfo.Valid {
			uaString = t.DeviceInfo.String
		} else {
			uaString = "Unknown"
		}

		// Parse the User Agent string
		ua := user_agent.New(uaString)
		browserName, browserVer := ua.Browser()

		isCurrent := (uaString == currentUA)

		devices = append(devices, DeviceResponse{
			TokenID:      t.ID,
			DeviceName:   ua.Platform(), // returns "iPad", "Windows", etc.
			Browser:      browserName + " " + browserVer,
			OS:           ua.OS(),
			RawUserAgent: uaString,
			LoginTime:    t.CreatedAt.Time,
			ExpiresAt:    t.ExpiresAt.Time,
			IsCurrent:    isCurrent,
		})
	}

	// 3. Respond
	// If devices is nil (empty), we return an empty slice [] instead of null
	if devices == nil {
		devices = []DeviceResponse{}
	}

	helpers.LogInfo("GetUserDevicesHandler", "devices listed", "user_id", userId, "count", len(devices))
	helpers.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"devices": devices,
	})
}
