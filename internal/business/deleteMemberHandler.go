package business

import (
	"errors" // <-- Needed for big.NewInt check or Sign()
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) DeleteMemberHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Auth Checks
	loggedUserRole, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}

	if loggedUserRole == 2 {
		helpers.LogInfo("DeleteMemberHandler", "access denied: employee tried to delete member")
		helpers.RespondWithError(w, http.StatusForbidden, "Access denied: Employees cannot delete members")
		return
	}

	targetUserID, ok := auth.ExtractUUID(w, r, "member_id")
	if !ok {
		return
	}

	// 3. Self-Deletion Check
	loggedUserId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}

	if loggedUserId.Bytes == targetUserID.Bytes {
		helpers.LogInfo("DeleteMemberHandler", "self-deletion attempt blocked", "user_id", loggedUserId)
		helpers.RespondWithError(w, http.StatusForbidden, "Access denied: Members cannot delete themselves")
		return
	}

	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

	// 4. Get Target Status
	targetRoleBalance, err := h.db.GetMemberRoleBalance(r.Context(), db.GetMemberRoleBalanceParams{
		BusinessID: businessId,
		UserID:     targetUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.LogInfo("DeleteMemberHandler", "target user not found in business", "target_id", targetUserID.Bytes)
			helpers.RespondWithError(w, http.StatusNotFound, "Target user is not a member of this business")
			return
		}
		helpers.LogError("DeleteMemberHandler", "db error fetching member role", "err", err.Error())
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// 5. Admin Restrictions
	if loggedUserRole == 3 { // Admin
		if targetRoleBalance.Role == db.BusinessRoleCreator || targetRoleBalance.Role == db.BusinessRoleAdmin {
			helpers.LogInfo("DeleteMemberHandler", "admin tried to remove higher privilege user", "target_role", targetRoleBalance.Role)
			helpers.RespondWithError(w, http.StatusForbidden, "Admins cannot remove other Admins or Creators")
			return
		}
	}

	// Creator Protection (Universal Rule: Nobody deletes the Creator)
	if targetRoleBalance.Role == db.BusinessRoleCreator {
		helpers.LogInfo("DeleteMemberHandler", "attempt to remove creator blocked")
		helpers.RespondWithError(w, http.StatusForbidden, "The Business Creator cannot be removed.")
		return
	}

	// 6. Balance Check (The Bug Fix)
	// pgtype.Numeric uses Int *big.Int.
	// We check if the Sign() of the big int is not 0.
	// Sign returns: -1 if < 0, 0 if == 0, +1 if > 0
	if targetRoleBalance.CurrentBalance.Int.Sign() != 0 {
		helpers.LogInfo("DeleteMemberHandler", "deletion blocked: non-zero balance",
			"target_id", targetUserID.Bytes,
			"balance_int", targetRoleBalance.CurrentBalance.Int.String()) // Logging the raw int value for debug
		helpers.RespondWithError(w, http.StatusConflict, "Cannot remove member with non-zero balance. Settle accounts first.")
		return
	}

	// 7. Execute Delete
	if err := h.db.DeleteBusinessMember(r.Context(), db.DeleteBusinessMemberParams{
		BusinessID: businessId,
		UserID:     targetUserID,
	}); err != nil {
		helpers.LogError("DeleteMemberHandler", "db error delete member", "err", err.Error())
		helpers.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// 8. Success
	helpers.LogInfo("DeleteMemberHandler", "member deleted successfully", "target_user_id", targetUserID.Bytes)
	w.WriteHeader(http.StatusNoContent)
}
