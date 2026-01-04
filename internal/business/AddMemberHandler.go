package business

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/configs"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// AddMemberHandler adds a new user to a business with a specific role.
// Only the 'creator' or an 'admin' of the business can perform this action.
func (h *Handler) AddMemberHandler(w http.ResponseWriter, r *http.Request) {
	// if user is not admin|creator return at the beginning itself
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	if role == 2 {
		helpers.RespondWithError(w, http.StatusUnauthorized, "Not enough credentials")
		helpers.LogInfo("AddMemberHandler", "not an admin|creator")
		return
	}
	curPlan, ok := auth.GetCurPlanFromContext(w, r)
	if !ok {
		return
	}
	baseName := strings.Split(curPlan.CurrentPlanID.String, "-")[1]
	addOnInt, _ := strconv.Atoi(strings.Split(curPlan.CurrentPlanID.String, "-")[2])
	membersLimit := configs.Plans[baseName].UsersLimit + addOnInt
	if !(membersLimit > int(curPlan.MembersCount)) {
		helpers.RespondWithError(w, 403, "plan members limit reached")
		helpers.LogInfo("AddMemberHandler", "plan members limit reached")
		return
	}

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
		helpers.LogError("AddMemberHandler", "failed to decode request body", "error", err.Error())
		return
	}

	// 1. Get Requestor's ID
	requestorUuid, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}

	// Get Business ID from URL
	businessId, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid business ID")
		// CHANGED: Client error, log as Info.
		helpers.LogError("AddMemberHandler", "failed to parse business ID from URL", "error", err.Error(), "url_param", chi.URLParam(r, "id"))
		return
	}
	businessUuid := pgtype.UUID{
		Bytes: businessId,
		Valid: businessId != uuid.Nil,
	}

	// -------------------------------------------------------------------------------
	// 2. Add New Member
	// -------------------------------------------------------------------------------
	newUserId, err := uuid.Parse(body.UserId)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request: invalid user_id format")
		// CHANGED: Client error, log as Info.
		helpers.LogError("AddMemberHandler", "failed to parse new user ID from body", "error", err.Error(), "user_id_body", body.UserId)
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
		if helpers.IsUniqueViolation(err) {
			helpers.RespondWithError(w, http.StatusConflict, "User is already a member of this business")
			// ADDED: Log the conflict
			helpers.LogInfo("AddMemberHandler", "conflict: user already a member", "new_user_id", newUserId, "business_id", businessId)
			return
		}
		// Any other error is a real 500
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("AddMemberHandler", "db error in AddBusinessMember", "error", err.Error(), "new_user_id", newUserId)
		return
	}

	helpers.LogInfo("AddMemberHandler", "member added successfully", "new_user_id", newUserId, "business_id", businessId, "added_by", requestorUuid)
	helpers.RespondWithJSON(w, 201, resType{
		Member: member,
	})
}
