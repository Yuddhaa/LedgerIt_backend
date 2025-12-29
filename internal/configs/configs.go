package configs

import "time"

const (
	MONTHLY_ADDON = 7900
	YEARLY_ADDON  = 70800
	TRIAL_DAYS    = time.Duration(90 * 24 * time.Hour)
	FREE_PLAN_ID  = "permanent-solo-0"
)

// PlanStruct is type for individual plan information
type PlanStruct struct {
	Level            int `json:"level"`
	MonthlyBasePrice int `json:"monthly_base_price"`
	YearlyBasePrice  int `json:"yearly_base_price"`
	UsersLimit       int `json:"users_limit"`
}

// Plans gives details on each individual base plan
var Plans = map[string]PlanStruct{
	"solo":       {Level: 1, MonthlyBasePrice: 0, YearlyBasePrice: 0, UsersLimit: 1},
	"retail":     {Level: 2, MonthlyBasePrice: 14900, YearlyBasePrice: 118800, UsersLimit: 3},
	"wholesale":  {Level: 3, MonthlyBasePrice: 47900, YearlyBasePrice: 388800, UsersLimit: 8},
	"enterprice": {Level: 4, MonthlyBasePrice: 97400, YearlyBasePrice: 838800, UsersLimit: 15},
}
