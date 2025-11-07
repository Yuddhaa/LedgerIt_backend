package business

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
	r.Post("/", h.CreateBusinessHandler)
	return r
}

func (h *Handler) CreateBusinessHandler(w http.ResponseWriter, r *http.Request) {
	type reqType struct {
		Name string `json:"name"`
	}
	type resType struct {
		Business db.Business `json:"business"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request,"+err.Error())
		h.logger.Error("Bad Request in CreateBusinessHandler", "error", err)
		return
	}
	// extract userId from r.context
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		helpers.RespondWithError(w, 500, "could now retrieve claims from context in CreateBusinessHandler")
		h.logger.Error("could now retrieve claims from context in CreateBusinessHandler")
		return
	}
	userId, err := uuid.Parse(claims.UserId)
	if err != nil {
		helpers.RespondWithError(w, 500, "count not convert userId to uuid in CreateBusinessHandler,err:"+err.Error())
		h.logger.Error("count not convert userId to uuid in CreateBusinessHandler,err:", "err", err)
		return
	}

	// -------------------------------------------------------------------------------
	// begin transaction
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		helpers.RespondWithError(w, 500, "could not start transaction in CreateBusiness,err:"+err.Error())
		h.logger.Error("could not start transaction in CreateBusiness,err:" + err.Error())
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.db.WithTx(tx)

	userUuid := pgtype.UUID{
		Bytes: userId,
		Valid: userId != uuid.Nil,
	}

	// create business
	business, err := qtx.CreateBusiness(r.Context(), db.CreateBusinessParams{
		OwnerID: userUuid,
		Name:    body.Name,
	})
	if err != nil {
		helpers.RespondWithError(w, 500, "db error in CreateBusiness,err:"+err.Error())
		h.logger.Error("db error in CreateBusiness,err:", "err", err)
		return
	}

	// add the owner to business members.
	_, err = qtx.AddBusinessMember(r.Context(), db.AddBusinessMemberParams{
		UserID:     userUuid,
		BusinessID: business.ID,
		Role:       db.BusinessRole(db.BusinessRoleCreator),
	})
	if err != nil {
		helpers.RespondWithError(w, 500, "db error in AddBusinessMember,err:"+err.Error())
		h.logger.Error("db error in CreateBusiness,err:", "err", err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		helpers.RespondWithError(w, 500, "Error committing transaction")
		h.logger.Error("Failed to commit transaction in CreateBusinessHandler", "error", err)
		return
	}

	helpers.RespondWithJSON(w, 201, resType{
		Business: business,
	})
}
