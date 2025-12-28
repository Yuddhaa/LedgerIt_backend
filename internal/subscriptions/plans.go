package subscriptions

import (
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/configs"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

func (s *Handler) GetPlansHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: do something about currentPlan
	// todo: add is_offer_available
	type resType struct {
		CurrentPlan     db.GetBusinessCurrentPlanRow  `json:"current_plan,omitempty"`
		IsFreeAvailable bool                          `json:"is_free_available"`
		IsTrialAvilable bool                          `json:"is_trial_available"`
		IsOfferAvailabe bool                          `json:"is_offer_available"`
		MonthlyAddon    int                           `json:"monthly_addon"`
		YearlyAddon     int                           `json:"yearly_addon"`
		Plans           map[string]configs.PlanStruct `json:"base_plans"`
	}
	userId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}

	planEligibility, err := s.db.CheckUserPlanEligibility(r.Context(), userId)
	if err != nil {
		helpers.RespondWithError(w, 500, "internal server errro")
		helpers.LogError("GetPlansHandler", "db error in CheckUserPlanEligibility", "err", err.Error(), "userId", userId)
		return
	}
	res := resType{
		IsFreeAvailable: planEligibility.FreeAvailable,
		IsTrialAvilable: planEligibility.TrialAvailable,
		MonthlyAddon:    configs.MONTHLY_ADDON,
		YearlyAddon:     configs.YEARLY_ADDON,
		Plans:           configs.Plans,
	}

	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}
	// do a query to get the required data
	currentPlan, err := s.db.GetBusinessCurrentPlan(r.Context(), businessId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "Business not found")
			helpers.LogError("GetPlansHandler", "Business not found", "businessId", businessId)
			return
		}
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("GetPlansHandler", "db error in GetBusinessCurrentPlan", "err", err.Error(), "businessId", businessId)
		return
	}
	// add it to res.CurrentPlan
	res.CurrentPlan = currentPlan

	helpers.RespondWithJSON(w, 200, res)
	helpers.LogInfo("GetPlansHandler", "plans sent")
}
