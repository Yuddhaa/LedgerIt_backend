package subscriptions

import (
	"encoding/json"
	"errors"
	"net/http"

	"LedgerIt/internal/configs"
	"LedgerIt/internal/helpers"

	"github.com/jackc/pgx/v5"
)

func (h *Handler) ValidateOfferCode(w http.ResponseWriter, r *http.Request) {
	type reqBody struct {
		OfferCode string `json:"offer_code"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		helpers.LogError("ValidateOfferCode", "bad request", "r.body", r.Body)
		helpers.RespondWithError(w, http.StatusBadRequest, "bad request")
		return
	}
	marketer, err := h.db.GetOfferDetailsByCode(r.Context(), body.OfferCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			helpers.RespondWithError(w, http.StatusNotFound, "no offers found")
			helpers.LogInfo("ValidateOfferCode", "no offers found", "offercode", body.OfferCode)
			return
		}
		helpers.RespondWithError(w, http.StatusNotFound, "internal server error")
		helpers.LogInfo("ValidateOfferCode", "err in GetOfferDetailsByCode", "err", err.Error())
		return
	}
	discout := int(marketer.DiscountPercent)

	//(Price * Percentage) / 100
	monthlyAddon := (configs.MONTHLY_ADDON * (100 - discout)) / 100
	yearlyAddon := (configs.YEARLY_ADDON * (100 - discout)) / 100
	plans := make(map[string]configs.PlanStruct, len(configs.Plans))
	for k, v := range configs.Plans {
		// v is a copy, so modifying it is safe for the new map
		v.MonthlyBasePrice = (v.MonthlyBasePrice * (100 - discout)) / 100
		v.YearlyBasePrice = (v.YearlyBasePrice * (100 - discout)) / 100
		plans[k] = v
	}

	if ok := h.getPlans(w, r, monthlyAddon, yearlyAddon, plans); !ok {
		return
	}
	helpers.LogInfo("ValidateOfferCode", "offer validated")
}
