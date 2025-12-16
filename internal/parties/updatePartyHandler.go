package parties

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) UpdatePartyHandler(w http.ResponseWriter, r *http.Request) {
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
		helpers.RespondWithError(w, 400, "Bad Request")
		helpers.LogError("UpdatePartyHandler", "bad request body", "err", err.Error())
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	partyId, ok := auth.ExtractUUID(w, r, "party_id")
	if !ok {
		return
	}
	party, err := h.db.UpdateParty(r.Context(), db.UpdatePartyParams{
		ID:    partyId,
		Name:  body.Name,
		Place: strings.ToLower(body.Place),
		PhoneNumber: pgtype.Text{
			String: body.PhoneNumber,
			Valid:  body.PhoneNumber != "",
		},
		BusinessID: businessId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "Party not found")
			helpers.LogInfo("UpdatePartyHandler", "party not found in UpdateParty", "party_id", partyId)
			return
		}
		if helpers.IsUniqueViolation(err) {
			helpers.RespondWithError(w, http.StatusConflict, "party already exists")
			// ADDED: Log the conflict
			helpers.LogInfo("UpdatePartyHandler", "conflict: party already exists", "party name", body.Name, "business_id", businessId)
			return
		}
		helpers.RespondWithError(w, 500, "Internal server error")
		helpers.LogError("UpdatePartyHandler", "Db error in UpdateParty", "err", err.Error(), "body", body)
		return
	}
	res := resType{Party: party}
	helpers.LogInfo("UpdatePartyHandler", "response sent", "res", res)
	helpers.RespondWithJSON(w, 200, res)
}
