package server

import (
	"errors"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

const (
	MONTHLY_ADDON = 79
	YEARLY_ADDON  = 59
)

type planStruct struct {
	MonthlyBasePrice int `json:"monthly_base_price"`
	YearlyBasePrice  int `json:"yearly_base_price"`
	UsersLimit       int `json:"users_limit"`
}

// plans gives details on each individual base plan
var plans = map[string]planStruct{
	"solo":       {MonthlyBasePrice: 0, YearlyBasePrice: 0, UsersLimit: 1},
	"retail":     {MonthlyBasePrice: 149, YearlyBasePrice: 99, UsersLimit: 3},
	"wholesale":  {MonthlyBasePrice: 479, YearlyBasePrice: 324, UsersLimit: 8},
	"enterprice": {MonthlyBasePrice: 974, YearlyBasePrice: 699, UsersLimit: 15},
}

func (s *Server) GetPlansHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: do something about currentPlan
	type resType struct {
		CurrentPlan   db.GetBusinessCurrentPlanRow `json:"current_plan,omitempty"`
		FreeAvailable bool                         `json:"free_available"`
		TrialAvilable bool                         `json:"trial_available"`
		MonthlyAddon  int                          `json:"monthly_addon"`
		YearlyAddon   int                          `json:"yearly_addon"`
		Plans         map[string]planStruct        `json:"base_plans"`
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
		FreeAvailable: planEligibility.FreeAvailable,
		TrialAvilable: planEligibility.TrialAvailable,
		MonthlyAddon:  MONTHLY_ADDON,
		YearlyAddon:   YEARLY_ADDON,
		Plans:         plans,
	}

	if businessIdQuery := r.URL.Query().Get("business_id"); businessIdQuery != "" {
		businessId, ok := helpers.StringToUUID(w, r, "businessId", businessIdQuery)
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
	}

	helpers.RespondWithJSON(w, 200, res)
	helpers.LogInfo("GetPlansHandler", "plans sent")
}
