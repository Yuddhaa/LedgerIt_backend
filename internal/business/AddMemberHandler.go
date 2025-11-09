package business

import (
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// AddMemberHandler adds a new user to a business with a specific role.
// Only the 'creator' or an 'admin' of the business can perform this action.
func (h *Handler) AddMemberHandler(w http.ResponseWriter, r *http.Request) {
	type reqType struct {
		UserId string          `json:"user_id"`
		Role   db.BusinessRole `json:"role"`
	}
	type resType struct {
		Member db.BusinessMember `json:"member"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid JSON")
		// CHANGED: Client error, log as Info.
		helpers.LogInfo("AddMemberHandler", "failed to decode request body", "error", err)
		return
	}

	// 1. Get Requestor's ID
	requestorId, ok := h.getUserIDFromContext(w, r)
	if !ok {
		return // error and response already sent
	}
	requestorUuid := pgtype.UUID{
		Bytes: requestorId,
		Valid: requestorId != uuid.Nil,
	}

	// Get Business ID from URL
	businessId, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid business ID")
		// CHANGED: Client error, log as Info.
		helpers.LogInfo("AddMemberHandler", "failed to parse business ID from URL", "error", err, "url_param", chi.URLParam(r, "id"))
		return
	}
	businessUuid := pgtype.UUID{
		Bytes: businessId,
		Valid: businessId != uuid.Nil,
	}

	// -------------------------------------------------------------------------------
	// 2. Authorization Check
	// -------------------------------------------------------------------------------
	role, err := h.db.GetMemberRole(r.Context(), db.GetMemberRoleParams{
		UserID:     requestorUuid,
		BusinessID: businessUuid,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusForbidden, "Access denied")
			// ADDED: Log the failed authz check
			helpers.LogInfo("AddMemberHandler", "authz failed: requestor not a member", "requestor_id", requestorId, "business_id", businessId)
			return
		}
		// Any other error is a real 500
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("AddMemberHandler", "db error in GetMemberRole", "error", err, "requestor_id", requestorId)
		return
	}

	// A safer "deny-by-default" check
	if role != db.BusinessRoleCreator && role != db.BusinessRoleAdmin {
		helpers.RespondWithError(w, http.StatusForbidden, "Not authorized to add members")
		// ADDED: Log the failed authz check
		helpers.LogInfo("AddMemberHandler", "authz failed: insufficient role", "requestor_id", requestorId, "role", role)
		return
	}
	// -------------------------------------------------------------------------------
	// 3. Add New Member
	// -------------------------------------------------------------------------------
	newUserId, err := uuid.Parse(body.UserId)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid user_id format")
		// CHANGED: Client error, log as Info.
		helpers.LogInfo("AddMemberHandler", "failed to parse new user ID from body", "error", err, "user_id_body", body.UserId)
		return
	}
	member, err := h.db.AddBusinessMember(r.Context(), db.AddBusinessMemberParams{
		UserID: pgtype.UUID{
			Bytes: newUserId,
			Valid: newUserId != uuid.Nil,
		},
		BusinessID: businessUuid,
		Role:       body.Role,
	})
	if err != nil {
		if isUniqueViolation(err) {
			helpers.RespondWithError(w, http.StatusConflict, "User is already a member of this business")
			// ADDED: Log the conflict
			helpers.LogInfo("AddMemberHandler", "conflict: user already a member", "new_user_id", newUserId, "business_id", businessId)
			return
		}
		// Any other error is a real 500
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("AddMemberHandler", "db error in AddBusinessMember", "error", err, "new_user_id", newUserId)
		return
	}

	// ADDED: Log successful add
	helpers.LogInfo("AddMemberHandler", "member added successfully", "new_user_id", newUserId, "business_id", businessId, "added_by", requestorId)
	helpers.RespondWithJSON(w, 201, resType{
		Member: member,
	})
}
