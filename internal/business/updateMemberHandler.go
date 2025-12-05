package business

import (
	"encoding/json"
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) UpdateMemberHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Get Requester's Role
	requesterRole, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}

	// 2. Authorization Check (Early Exit)
	// Role 2 = Employee/Member. They cannot update anyone.
	if requesterRole == 2 {
		helpers.RespondWithError(w, http.StatusForbidden, "Access denied: Employees cannot update members")
		helpers.LogError("UpdateMemberHandler", "Access denied: Employees cannot update members")
		return
	}

	// 3. Parse Request Body
	type reqType struct {
		Role   string `json:"role"`
		UserId string `json:"user_id"`
	}
	type resType struct {
		BusinessMember db.BusinessMember `json:"business_member"`
	}
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad request: Invalid JSON")
		return
	}

	// 4. Validate Input Role
	newRole := db.BusinessRole(body.Role)
	if newRole != db.BusinessRoleAdmin && newRole != db.BusinessRoleEmployee && newRole != db.BusinessRoleCreator {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad request: Invalid role")
		return
	}

	// 5. Parse Target User ID
	targetUUID, err := uuid.Parse(body.UserId)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad request: Invalid user_id format")
		return
	}
	targetUserID := pgtype.UUID{Bytes: targetUUID, Valid: true}

	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

	// 6. Get Target Member's Current Status
	// We need to know who we are trying to update to enforce rules.
	targetCurrentRole, err := h.db.GetMemberRole(r.Context(), db.GetMemberRoleParams{
		BusinessID: businessId,
		UserID:     targetUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "Target user is not a member of this business")
			return
		}
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		helpers.LogError("UpdateMemberHandler", "db error fetching member role", "err", err)
		return
	}

	// 7. Admin Logic Restrictions (Role 3 = Admin)
	if requesterRole == 3 {
		// Rule A: Admin cannot update a Creator or another Admin
		if targetCurrentRole == db.BusinessRoleCreator || targetCurrentRole == db.BusinessRoleAdmin {
			helpers.RespondWithError(w, http.StatusForbidden, "Admins cannot modify other Admins or Creators")
			return
		}
		// Rule B: Admin cannot promote someone to Creator
		if newRole == db.BusinessRoleCreator {
			helpers.RespondWithError(w, http.StatusForbidden, "Admins cannot promote users to Creator")
			return
		}
	}

	// 8. Execute Update
	updatedMember, err := h.db.UpdateBusinessMember(r.Context(), db.UpdateBusinessMemberParams{
		Role:       newRole,
		BusinessID: businessId,
		UserID:     targetUserID,
	})
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		helpers.LogError("UpdateMemberHandler", "db error updating member", "err", err)
		return
	}

	// 9. Response
	res := resType{BusinessMember: updatedMember}

	// Log success
	helpers.LogInfo("UpdateMemberHandler", "member role updated", "target_user_id", targetUUID, "new_role", newRole)
	helpers.RespondWithJSON(w, http.StatusOK, res)
}
