package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/configs"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	razorpay "github.com/razorpay/razorpay-go"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Handler struct {
	db        *db.Queries
	pool      *pgxpool.Pool
	rp_client *razorpay.Client
}

var (
	PERIOD    = []string{"monthly", "yearly", "permanent"}
	BASEPLANS = []string{"solo", "retail", "wholesale", "enterprice"}
)

// reqType is used by /create and /update as a type for the r.Body
type reqType struct {
	Period      string `json:"period"`
	BasePlan    string `json:"base_plan"`
	AddOn       string `json:"add_on"`
	IsTrialing  bool   `json:"is_trialing,omitempty"`
	OfferCode   string `json:"offer_code,omitempty"`
	IsImmediate bool   `json:"is_immediate,omitempty"`
}

// resType is used by /create and /update as a type to send responses
type resType struct {
	ShortUrl       string      `json:"short_url"`
	SubscriptionId pgtype.UUID `json:"subscription_id"`
	RazorpaySubId  string      `json:"razorpay_sub_id"`
}

func NewHandler(db *db.Queries, pool *pgxpool.Pool) (*Handler, error) {
	razorpayKey := os.Getenv("RAZORPAY_API_KEY")
	if razorpayKey == "" {
		helpers.LogError("subscriptions.NewHandler", "RAZORPAY_API_KEY is not set in environment")
		return nil, fmt.Errorf("RAZORPAY_API_KEY is not set")
	}
	razorpaySecret := os.Getenv("RAZORPAY_API_SECRET")
	if razorpaySecret == "" {
		helpers.LogError("subscriptions.NewHandler", "RAZORPAY_API_SECRET is not set in environment")
		return nil, fmt.Errorf("RAZORPAY_API_SECRET is not set")
	}
	rp_client := razorpay.NewClient(razorpayKey, razorpaySecret)
	return &Handler{
		db:        db,
		pool:      pool,
		rp_client: rp_client,
	}, nil
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(h.GetRole)
	r.Get("/plans", h.GetPlansHandler)
	r.Post("/create", h.CreateHandler)
	r.Post("/update", h.UpdateHandler)
	return r
}

// Getrole returns int corresponding to the role as below
// -1 - err
// 0 - not a member => for these 2 automatically the middleware returns respective status code
// 1 - creator
// 2 - employee
// 3 - admin
func (h *Handler) GetRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roleInt := -1
		userId, ok := auth.GetUserIdFromContext(w, r)
		if !ok {
			return
		}
		businessId, ok := auth.ExtractUUID(w, r, "id")
		if !ok {
			return
		}
		role, err := h.db.GetUserRole(r.Context(), db.GetUserRoleParams{
			UserID:     userId,
			BusinessID: businessId,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				helpers.RespondWithError(w, http.StatusUnauthorized, "no user found")
				helpers.LogInfo("GetRole", "no user found", "userId", userId, "businessId", businessId)
				roleInt = 0
				return
			}
			helpers.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
			helpers.LogError("getrole", "db error in GetUserRole", "error", err.Error(), "userId", userId, "businessId", businessId)
			roleInt = -1
			return
		}
		switch role {
		case db.BusinessRoleEmployee:
			roleInt = 2 // employee
		case db.BusinessRoleAdmin:
			roleInt = 3 // admin
		default:
			roleInt = 1 // creator
		}

		ctx := context.WithValue(r.Context(), "role", roleInt)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validateRequestBody validates the body in /crate and /update handler
func validateRequestBody(w http.ResponseWriter, body reqType) bool {
	isBadRequest := !slices.Contains(PERIOD, body.Period)
	if isBadRequest {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request:bad period")
		helpers.LogInfo("validateRequestBody", "bad 'type' in body")
		return false
	}
	isBadRequest = !slices.Contains(BASEPLANS, body.BasePlan)
	if isBadRequest {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request:bad base plan")
		helpers.LogInfo("validateRequestBody", "bad 'base_plan' in body")
		return false
	}
	if body.Period == "permanent" && !(body.BasePlan == "solo") {
		helpers.RespondWithError(w, http.StatusBadRequest, "Bad Request:period=permanent can only be used with solo")
		helpers.LogInfo("validateRequestBody", "Bad Request:period=permanent can only be used with solo")
		return false
	}

	return true
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
		helpers.LogInfo("getOrCreatePlan", "bad 'add_on' in body")
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
				helpers.LogError("getOrCreatePlan", configs.FREE_PLAN_ID+" plan is missing from plans table")
				return db.Plan{}, false
			}
			// ********************************************************************************************************
			// create a razorpay plan first
			name := fmt.Sprintf("%v with %v additional members", body.BasePlan, body.AddOn)
			description := fmt.Sprintf("%s %s Plan with %s additional users.", cases.Title(language.English).String(body.Period), cases.Title(language.English).String(body.BasePlan), body.AddOn)
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
				helpers.LogError("getOrCreatePlan", "error in creating the plan", "err", err.Error())
				return db.Plan{}, false
			}
			razorpayPlanId, ok := newRazorpayPlan["id"].(string)
			if !ok {
				helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
				helpers.LogError("getOrCreatePlan", "error in asserting the razorpayPlanId form any to string")
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
				helpers.LogError("getOrCreatePlan", "db error in CreatePlan", "err", err.Error(), "planId", planId)
				return db.Plan{}, false
			}
		} else {
			helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
			helpers.LogError("getOrCreatePlan", "db error in GetPlan", "err", err.Error(), "planId", planId)
			return db.Plan{}, false
		}
	}
	return plan, true
}

func (h *Handler) createSubscription(w http.ResponseWriter, r *http.Request, body reqType,
	plan db.Plan, notes map[string]any, businessId pgtype.UUID, startAt *int64,
) (resType, bool) {
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
		"notes":           notes,
	}
	if startAt != nil {
		newSubscriptionData["start_at"] = *startAt
	}
	newRazorpaySub, err := h.rp_client.Subscription.Create(newSubscriptionData, nil)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("CreateHandler", "error in creating razorpay subscription", "err", err.Error(), "planId", plan.ID)
		return resType{}, false
	}
	razorpaySubId, ok := newRazorpaySub["id"].(string)
	if !ok {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("CreateHandler", "can't convert razorpaySubId to go string", "subscription id", newRazorpaySub["id"])
		return resType{}, false
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
		return resType{}, false
	}
	// ****************************************************************************************************************
	// send response
	// ****************************************************************************************************************
	shortUrl, ok := newRazorpaySub["short_url"].(string) // Defaults to "" if missing, no panic
	if !ok {
		helpers.RespondWithError(w, 500, "internal server error")
		helpers.LogError("CreateHandler", "can't convert short_url to string")
		return resType{}, false
	}
	return resType{
		ShortUrl:       shortUrl,
		SubscriptionId: subscription.ID,
		RazorpaySubId:  razorpaySubId,
	}, true
}

// handleSoloPlan if the user is allowed a solo plan (i.e. if doesn't already have one solo business)
// will create a solo plan
func (h *Handler) handleSoloPlan(w http.ResponseWriter, r *http.Request, body reqType,
	userId, businessId pgtype.UUID, update bool, curPlan db.GetBusinessCurrentPlanRow,
) {
	// ****************************************************************************************************************
	// check if either solo or trial available before moving forward if thats what the user has clicked
	// ****************************************************************************************************************
	trialOrSoloAllowed, err := h.db.CheckUserPlanEligibility(r.Context(), userId)
	if err != nil {
		helpers.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		helpers.LogError("CreateHandler", "db error in CheckUserPlanEligibility", "err", err.Error())
		return
	}
	if body.BasePlan == "solo" && !trialOrSoloAllowed.FreeAvailable {
		helpers.RespondWithError(w, http.StatusForbidden, "Free business already exists")
		helpers.LogInfo("CreateHandler", "Free business already exists", "businessId", businessId)
		return
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
		if update {
			// if subscription already exists cancel that first
			if ok := h.cancelSubscription(w, curPlan); !ok {
				return
			}
		}
		err := h.db.UpdateBusinessSubscription(r.Context(), db.UpdateBusinessSubscriptionParams{
			ID:                    businessId,
			CurrentPlanID:         pgtype.Text{String: plan.ID, Valid: true},
			SubscriptionsStatus:   db.NullSubscriptionsStatus{SubscriptionsStatus: db.SubscriptionsStatusActive, Valid: true},
			IsTrialUsed:           pgtype.Bool{Valid: false},
			CurrentSubscriptionID: pgtype.UUID{Valid: false},
			IsOfferUsed:           pgtype.Bool{Valid: false},
			SubscriptionEndPeriod: pgtype.Timestamptz{
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
	}
}

func (h *Handler) cancelSubscription(w http.ResponseWriter, curPlan db.GetBusinessCurrentPlanRow) bool {
	if curPlan.RazorpaySubscriptionID.Valid && curPlan.RazorpaySubscriptionID.String != "" {
		rzpID := curPlan.RazorpaySubscriptionID.String

		// "cancel_at_cycle_end": 0 means "kill it now"
		_, err := h.rp_client.Subscription.Cancel(rzpID, map[string]any{"cancel_at_cycle_end": 0}, nil)
		if err != nil {
			// 3. SMART ERROR HANDLING

			// Case A: If it's already cancelled (or doesn't exist), we can proceed safely.
			// Razorpay error messages usually contain "BAD_REQUEST_ERROR" or descriptions like "Subscription is not in active state"
			if strings.Contains(err.Error(), "BAD_REQUEST_ERROR") || strings.Contains(err.Error(), "not in active state") {
				helpers.LogInfo("UpdateHandler", "Subscription was already inactive, proceeding to downgrade", "subID", rzpID)
			} else {
				// Case B: Network/Server error (The Dangerous Ones)
				// STOP HERE. Do not update DB.
				helpers.LogError("UpdateHandler", "Failed to cancel Razorpay sub - Aborting Downgrade",
					"subID", rzpID, "err", err.Error())
				helpers.RespondWithError(w, http.StatusBadGateway, "Failed to cancel subscription with bank. Please try again.")
				return false
			}
		} else {
			helpers.LogInfo("UpdateHandler", "Old subscription cancelled for downgrade", "subID", rzpID)
		}
	}
	return true
}
