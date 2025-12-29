package subscriptions

import (
	"errors"
	"net/http"
	"strings"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/configs"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

func (s *Handler) GetPlansHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Define JSON Response Structures
	// We use pointers (*string, *int64) so that if the DB value is NULL,
	// the JSON output will be "null" instead of empty string "" or 0.
	type currentPlanDetails struct {
		PlanID                 *string `json:"plan_id"`
		BasePlan               string  `json:"base_plan"` // parsed from ID
		AddOn                  string  `json:"add_on"`    // parsed from ID
		Period                 string  `json:"period"`
		MembersCount           int32   `json:"members_count"`
		Status                 any     `json:"status"` // string or null
		Name                   *string `json:"name"`
		Amount                 *int64  `json:"amount"`
		Currency               *string `json:"currency"`
		SubscriptionEndPeriod  any     `json:"subscription_end_period"`
		SubscriptionID         *string `json:"subscription_id"`
		RazorpaySubscriptionID *string `json:"razorpay_subscription_id"`
	}

	type resType struct {
		CurrentPlan      *currentPlanDetails           `json:"current_plan"` // Pointer allows null
		IsFreeAvailable  bool                          `json:"is_free_available"`
		IsTrialAvailable bool                          `json:"is_trial_available"`
		IsOfferAvailable bool                          `json:"is_offer_available"`
		MonthlyAddon     int                           `json:"monthly_addon"`
		YearlyAddon      int                           `json:"yearly_addon"`
		Plans            map[string]configs.PlanStruct `json:"base_plans"`
	}

	// 2. Auth & Context
	userId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

	// 3. Check Eligibility
	planEligibility, err := s.db.CheckUserPlanEligibility(r.Context(), userId)
	if err != nil {
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("GetPlansHandler", "db error in CheckUserPlanEligibility", "err", err.Error())
		return
	}

	// 4. Fetch Current Plan Data
	dbPlan, err := s.db.GetBusinessCurrentPlan(r.Context(), businessId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "Business not found")
			helpers.LogInfo("GetPlansHandler", "Business not found", "businessId", businessId)
			return
		}
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("GetPlansHandler", "db error fetching plan", "err", err.Error())
		return
	}

	// 5. Construct Base Response
	res := resType{
		IsFreeAvailable:  planEligibility.FreeAvailable,
		IsTrialAvailable: planEligibility.TrialAvailable,
		IsOfferAvailable: false, // Placeholder for future logic
		MonthlyAddon:     configs.MONTHLY_ADDON,
		YearlyAddon:      configs.YEARLY_ADDON,
		Plans:            configs.Plans,
	}

	// 6. Populate Current Plan (Only if a plan exists)
	if dbPlan.CurrentPlanID.Valid {
		// A. Parse Plan ID (e.g., "monthly-retail-5" -> base="retail", addon=5)
		parts := strings.Split(dbPlan.CurrentPlanID.String, "-")
		period := parts[0]
		basePlan := parts[1]
		addon := parts[2]

		// B. Safely Convert DB Types to JSON Types

		// UUID -> String
		var subIDStr *string
		if dbPlan.SubscriptionID.Valid {
			s := dbPlan.SubscriptionID.String()
			subIDStr = &s
		}

		// pgtype.Text -> String
		var rzpSubIDStr *string
		if dbPlan.RazorpaySubscriptionID.Valid {
			s := dbPlan.RazorpaySubscriptionID.String
			rzpSubIDStr = &s
		}

		// Enum -> String
		var statusStr any = nil
		if dbPlan.SubscriptionsStatus.Valid {
			statusStr = dbPlan.SubscriptionsStatus.SubscriptionsStatus
		}

		// C. Build the Object
		res.CurrentPlan = &currentPlanDetails{
			PlanID:                 &dbPlan.CurrentPlanID.String,
			Period:                 period,
			BasePlan:               basePlan,
			AddOn:                  addon,
			MembersCount:           dbPlan.MembersCount,
			Status:                 statusStr,
			Name:                   &dbPlan.PlanName.String,
			Amount:                 &dbPlan.Amount.Int64,
			Currency:               &dbPlan.Currency.String,
			SubscriptionEndPeriod:  dbPlan.SubscriptionEndPeriod, // pgtype handles JSON marshaling well
			SubscriptionID:         subIDStr,
			RazorpaySubscriptionID: rzpSubIDStr,
		}
	} else {
		// No plan assigned
		res.CurrentPlan = nil
	}

	helpers.RespondWithJSON(w, 200, res)
	helpers.LogInfo("GetPlansHandler", "plans sent")
}
