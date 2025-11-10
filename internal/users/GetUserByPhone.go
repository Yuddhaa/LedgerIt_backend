package users

import (
	"errors"
	"net/http"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) GetUserByPhone(w http.ResponseWriter, r *http.Request) {
	phone_no := chi.URLParam(r, "phone_no")
	type resType struct {
		User db.User `json:"user"`
	}
	user, err := h.db.GetUserByPhone(r.Context(), pgtype.Text{
		String: phone_no,
		Valid:  phone_no != "",
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "user not found")
			helpers.LogInfo("GetUserByPhone", "User not found", "phone_no", phone_no)
			return
		}

		// All other errors are actual server errors
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("GetUserByPhone", "Could not retrieve user by phone_no", "error", err, "phone_no", phone_no)
		return
	}
	res := resType{
		User: user,
	}
	helpers.LogInfo("GetUserByPhone", "response sent", "response", res)
	helpers.RespondWithJSON(w, 200, res)
}
