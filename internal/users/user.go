package users

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db     *db.Queries
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewHandler(db *db.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{
		db:     db,
		pool:   pool,
		logger: logger,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Put("/me", h.UpdateUserProfileHandler)
	return r
}

func (h *Handler) UpdateUserProfileHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		helpers.RespondWithError(w, 500, "could now retrieve claims from context")
		h.logger.Error("could now retrieve claims from context")
		return
	}
	userId := claims.UserId
	userUuid, err := uuid.Parse(userId)
	if err != nil {
		helpers.RespondWithError(w, 500, "userId to uuid parse err")
		h.logger.Error("userId to uuid parse err", "error", err)
		return
	}
	type reqType struct {
		Name        string `json:"name"`
		PhoneNumber string `json:"phoneNumber"`
	}
	type resType struct {
		User db.User `json:"user"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request,"+err.Error())
		h.logger.Error("Bad Request in UpdateUserProfile", "error", err)
		return
	}
	helpers.PrintResponse("body in UpdateUserProfileHandler", body)

	user, err := h.db.UpdateUserProfile(r.Context(), db.UpdateUserProfileParams{
		ID: pgtype.UUID{
			Bytes: userUuid,
			Valid: userUuid != uuid.Nil,
		},
		Name: pgtype.Text{
			String: body.Name,
			Valid:  body.Name != "",
		},
		PhoneNumber: pgtype.Text{
			String: body.PhoneNumber,
			Valid:  body.PhoneNumber != "",
		},
	})
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "error in sql UpdateUserProfile"+err.Error())
		h.logger.Error("error in sql UpdateUserProfile", "error", err)
		return
	}

	helpers.PrintResponse("response in UpdateUserProfileHandler", resType{
		User: user,
	})
	helpers.RespondWithJSON(w, 200, resType{
		User: user,
	})
}
