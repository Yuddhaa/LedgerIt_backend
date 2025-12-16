package parties

import (
	"encoding/json"
	"net/http"
	"strings"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) AddPartyHandler(w http.ResponseWriter, r *http.Request) {
	type reqType struct {
		Name        string `json:"name"`
		Place       string `json:"place"`
		PhoneNumber string `json:"phone_number"`
	}
	type resType struct {
		Party db.Party `json:"party"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad request")
		helpers.LogError("AddPartyHandler", "json decode error", "err", err.Error())
		return
	}
	helpers.LogInfo("AddPartyHandler", "req got", "req body", body)
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

	if body.Place == "" {
		helpers.RespondWithError(w, http.StatusBadRequest, "Please provide place")
		helpers.LogInfo("AddPartyHandler", "place is empty")
		return
	}

	party, err := h.db.CreateParty(r.Context(), db.CreatePartyParams{
		Name:       body.Name,
		Place:      strings.ToLower(body.Place),
		BusinessID: businessId,
		PhoneNumber: pgtype.Text{
			String: body.PhoneNumber,
			Valid:  body.PhoneNumber != "",
		},
	})
	if err != nil {
		if helpers.IsUniqueViolation(err) {
			helpers.RespondWithError(w, http.StatusConflict, "party already exists")
			// ADDED: Log the conflict
			helpers.LogInfo("AddPartyHandler", "conflict: party already exists", "party name", body.Name, "business_id", businessId)
			return
		}
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("AddPartyHandler", "error in db CreateParty", "err", err.Error())
		return
	}
	res := resType{Party: party}
	helpers.LogInfo("AddPartyHandler", "response sent", "response", res)
	helpers.RespondWithJSON(w, 201, res)
}
