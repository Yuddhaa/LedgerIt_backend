package subscriptions

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/configs"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	PERIOD    = []string{"monthly", "yearly", "permanent"}
	BASEPLANS = []string{"solo", "retail", "wholesale", "enterprice"}
)

type reqType struct {
	Period     string `json:"period"`
	BasePlan   string `json:"base_plan"`
	AddOn      string `json:"add_on"`
	IsTrialing bool   `json:"is_trialing"`
	OfferCode  string `json:"offer_code"`
}

// CreateHandler creates a plan(if not exists) and
// creates+returns corresponding subscriptions
func (h *Handler) CreateHandler(w http.ResponseWriter, r *http.Request) {
	type resType struct {
		ShortUrl       string      `json:"short_url"`
		SubscriptionId pgtype.UUID `json:"subscription_id"`
		RazorpaySubId  string      `json:"razorpay_sub_id"`
	}
	// ****************************************************************************************************************
	// get loggedin userInfo
	// ****************************************************************************************************************
	userId, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		return
	}
	businessId, ok := auth.ExtractUUID(w, r, "id")
	if !ok {
		return
	}

	// if not creator -> get out
	role, ok := auth.GetUserRoleFromContext(w, r)
	if !ok {
		return
	}
	if role != 1 {
		helpers.RespondWithError(w, http.StatusForbidden, "Only creator can create subscriptions")
		helpers.LogError("CreateHandler", "Only creator can create subscriptions", "role", role, "userId", userId)
		return
	}

	// ****************************************************************************************************************
	// if creator, decode the body and validate the parameters
	// ****************************************************************************************************************
	var body reqType
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request")
		helpers.LogInfo("CreateHandler", "cannot decode body", "err", err.Error())
		return
	}
	isBadRequest := !slices.Contains(PERIOD, body.Period)
	if isBadRequest {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request:bad period")
		helpers.LogInfo("CreateHandler", "bad 'type' in body")
		return
	}
	isBadRequest = !slices.Contains(BASEPLANS, body.BasePlan)
	if isBadRequest {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request:bad base plan")
		helpers.LogInfo("CreateHandler", "bad 'base_plan' in body")
		return
	}
	if body.Period == "permanent" && !(body.BasePlan == "solo") {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request:period=permanent can only be used with solo")
		helpers.LogInfo("CreateHandler", "Bad Request:period=permanent can only be used with solo")
		return
	}

	// ****************************************************************************************************************
	// check if either solo or trial available before moving forward if thats what the user has clicked
	// ****************************************************************************************************************
	if body.IsTrialing || body.BasePlan == "solo" {
		trialOrSoloAllowed, err := h.db.CheckUserPlanEligibility(r.Context(), userId)
		if err != nil {
			helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
			helpers.LogError("CreateHandler", "db error in CheckUserPlanEligibility", "err", err.Error())
			return
		}
		if body.IsTrialing && !trialOrSoloAllowed.TrialAvailable {
			helpers.RespondWithError(w, http.StatusForbidden, "Trial is not available")
			helpers.LogInfo("CreateHandler", "Trial is not available", "businessId", businessId)
			return
		}
		if body.BasePlan == "solo" && !trialOrSoloAllowed.FreeAvailable {
			helpers.RespondWithError(w, http.StatusForbidden, "Free business already exists")
			helpers.LogInfo("CreateHandler", "Free business already exists", "businessId", businessId)
			return
		}
	}

	// ****************************************************************************************************************
	// get or create plan
	// ****************************************************************************************************************
	plan, ok := h.getOrCreatePlan(w, r, body)
	if !ok {
		return
	}
	// ****************************************************************************************************************
	// if the plan is free, we update the business table here only and return
	// no need to create subscription
	// ****************************************************************************************************************
	if plan.ID == configs.FREE_PLAN_ID {
		err := h.db.UpdateBusinessSubscription(r.Context(), db.UpdateBusinessSubscriptionParams{
			ID:                  businessId,
			CurrentPlanID:       pgtype.Text{String: plan.ID, Valid: true},
			SubscriptionsStatus: db.NullSubscriptionsStatus{SubscriptionsStatus: db.SubscriptionsStatusActive, Valid: true},
			IsTrialUsed:         pgtype.Bool{Valid: false},
			SubscriptionEndDate: pgtype.Timestamptz{
				Time:  time.Now().Add(100 * 364 * 24 * time.Hour), // 100 years
				Valid: true,
			},
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				helpers.LogInfo("CreateHandler", "business not found in UpdateBusinessSubscription")
				helpers.RespondWithError(w, http.StatusNotFound, "business not found")
				return
			}
			helpers.LogInfo("CreateHandler", "db error in UpdateBusinessSubscription", "err", err.Error(),
				"businessId", businessId)
			helpers.RespondWithError(w, http.StatusInternalServerError, "internal server errorr")
			return
		}
		helpers.LogInfo("CreateHandler", "free plan applied")
		helpers.RespondWithJSON(w, 200, "free plan applied")
		return
	}

	// ****************************************************************************************************************
	// now that we have the plan, create the subscription
	// ****************************************************************************************************************
	var totalCount int
	switch plan.Period {
	case db.PlansPeriodMonthly:
		totalCount = 120
	case db.PlansPeriodYearly:
		totalCount = 10
	}
	newSubscriptionData := map[string]any{
		"plan_id":         plan.RazorpayPlanID,
		"total_count":     totalCount,
		"customer_notify": 1,
		"notes": map[string]any{
			"userId":     userId,
			"businessId": businessId,
		},
	}
	// if this is a trial, set start 90 days from now
	if body.IsTrialing {
		newSubscriptionData["start_at"] = time.Now().Add(configs.TRIAL_DAYS).Unix()
	}
	newRazorpaySub, err := h.rp_client.Subscription.Create(newSubscriptionData, nil)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("CreateHandler", "error in creating razorpay subscription", "err", err.Error(), "planId", plan.ID,
			"businessId", businessId)
		return
	}
	razorpaySubId, ok := newRazorpaySub["id"].(string)
	if !ok {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("CreateHandler", "can't convert razorpaySubId to go string", "subscription id", newRazorpaySub["id"])
		return
	}
	// ****************************************************************************************************************
	// create a new subscriptions row
	createSubscriptionParams := db.CreateSubscriptionParams{
		BusinessID:             businessId,
		PlanID:                 plan.ID,
		RazorpaySubscriptionID: razorpaySubId,
		MarketerID: pgtype.UUID{
			Valid: false,
		},
		IsOfferApplied: false,
	}
	if body.IsTrialing {
		createSubscriptionParams.Status = db.SubscriptionsStatusTrialingPending
	} else {
		createSubscriptionParams.Status = db.SubscriptionsStatusPending
	}
	subscription, err := h.db.CreateSubscription(r.Context(), createSubscriptionParams)
	if err != nil {
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("CreateHandler", "db error in creating the subscription row", "err", err.Error())
		return
	}
	// ****************************************************************************************************************
	// send response
	// ****************************************************************************************************************
	shortUrl, ok := newRazorpaySub["short_url"].(string) // Defaults to "" if missing, no panic
	if !ok {
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("CreateHandler", "can't convert short_url to string")
		return
	}
	res := resType{
		ShortUrl:       shortUrl,
		SubscriptionId: subscription.ID,
		RazorpaySubId:  razorpaySubId,
	}
	helpers.RespondWithJSON(w, 200, res)
	helpers.LogInfo("CreateHandler", "subscription created", "res", res)
}

// getOrCreatePlan gets or creates plan
func (h *Handler) getOrCreatePlan(w http.ResponseWriter, r *http.Request, body reqType) (db.Plan, bool) {
	// ****************************************************************************************************************
	// construct planId and some other required parameters
	// ****************************************************************************************************************
	// if plan is solo, force the addon and period
	if body.BasePlan == "solo" {
		body.AddOn = "0"
		body.Period = "permanent"
	}
	addOnInt, err := strconv.Atoi(body.AddOn)
	if err != nil {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request:bad add on")
		helpers.LogInfo("CreateHandler", "bad 'add_on' in body")
		return db.Plan{}, false
	}
	planId := fmt.Sprintf("%s-%s-%d", body.Period, body.BasePlan, addOnInt)
	var amount int
	if body.Period == "yearly" {
		amount = configs.Plans[body.BasePlan].YearlyBasePrice + (addOnInt * configs.YEARLY_ADDON)
	} else {
		amount = configs.Plans[body.BasePlan].MonthlyBasePrice + (addOnInt * configs.MONTHLY_ADDON)
	}
	// ****************************************************************************************************************
	// get or create new plan
	// ****************************************************************************************************************
	plan, err := h.db.GetPlan(r.Context(), planId)
	if err != nil {
		// if plan doesn't exist create a new one
		if errors.Is(err, pgx.ErrNoRows) {
			if planId == configs.FREE_PLAN_ID {
				helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
				helpers.LogError("createRazorpayPlan", configs.FREE_PLAN_ID+" plan is missing from plans table")
				return db.Plan{}, false
			}
			// ********************************************************************************************************
			// create a razorpay plan first
			name := fmt.Sprintf("%v with %v additional members", body.BasePlan, body.AddOn)
			description := fmt.Sprintf("%s %s Plan with %d additional users.", cases.Title(language.English).String(body.Period), cases.Title(language.English).String(body.BasePlan), body.AddOn)
			// Output: "Monthly Retail Plan with 5 additional users."

			newPlanData := map[string]any{
				"period":   body.Period,
				"interval": 1,
				"item": map[string]any{
					"name":        name,
					"amount":      amount,
					"currency":    "INR",
					"description": description,
				},
				"notes": map[string]any{
					"base plan": body.BasePlan,
					"add on":    body.AddOn,
				},
			}
			newRazorpayPlan, err := h.rp_client.Plan.Create(newPlanData, nil)
			if err != nil {
				helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
				helpers.LogError("createRazorpayPlan", "error in creating the plan", "err", err.Error())
				return db.Plan{}, false
			}
			razorpayPlanId, ok := newRazorpayPlan["id"].(string)
			if !ok {
				helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
				helpers.LogError("createRazorpayPlan", "error in asserting the razorpayPlanId form any to string")
				return db.Plan{}, false
			}
			// ********************************************************************************************************
			// create a new plans row
			plan, err = h.db.CreatePlan(r.Context(), db.CreatePlanParams{
				ID:             planId,
				RazorpayPlanID: razorpayPlanId,
				Name:           name,
				Description: pgtype.Text{
					String: description,
					Valid:  true,
				},
				Amount:    int64(amount),
				Currency:  "INR",
				UserLimit: int32(configs.Plans[body.BasePlan].UsersLimit + addOnInt),
				Period:    db.PlansPeriod(body.Period),
				Active: pgtype.Bool{
					Bool:  true,
					Valid: true,
				},
			})
			if err != nil {
				helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
				helpers.LogError("CreateHandler", "db error in CreatePlan", "err", err.Error(), "planId", planId)
				return db.Plan{}, false
			}
		} else {
			helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
			helpers.LogError("CreateHandler", "db error in GetPlan", "err", err.Error(), "planId", planId)
			return db.Plan{}, false
		}
	}
	return plan, true
}
