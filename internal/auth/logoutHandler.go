package auth

import (
	"encoding/json"
	"net/http"

	"LedgerIt/internal/helpers"
)

func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// req and res type + decode body
	type reqType struct {
		RefreshToken string `json:"refreshToken"`
	}
	type resType struct {
		Msg string `json:"message"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request")
		h.logger.Error("bad request, err:" + err.Error())
		return
	}
	helpers.PrintResponse("LogoutHandler req.body:", body)
	// hash the refresh token
	hashedToken := hashToken(body.RefreshToken)
	// delete from refresh token table
	if err := h.db.DeleteRefreshTokenByHash(r.Context(), hashedToken); err != nil {
		helpers.RespondWithError(w, 500, "error in DeleteRefreshTokenByHash,err:"+err.Error())
		h.logger.Error("error in DeleteRefreshTokenByHash,err:" + err.Error())
		return
	}
	helpers.RespondWithJSON(w, 200, resType{Msg: "User logged out"})
}
