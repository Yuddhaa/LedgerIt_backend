package configs

import (
	"fmt"
	"os"
	"time"

	"LedgerIt/internal/helpers"
)

type configType struct {
	MODE                 string
	PORT                 string
	DBURL                string
	JWT_SECRET           string
	GOOGLE_WEB_CLIENT_ID string
	GOOGLE_ANDROID_ID    string
	GOOGLE_IOS_ID        string
	FIREBASE_CLIENT_ID   string
	RAZORPAY_API_KEY     string
	RAZORPAY_API_SECRET  string
}

var Configs configType

const (
	MONTHLY_ADDON = 7900
	YEARLY_ADDON  = 70800
	PERMANENT     = 1499900
	TRIAL_DAYS    = time.Duration(90 * 24 * time.Hour)
	// TRIAL_DAYS   = time.Duration(time.Minute * 10)
	FREE_PLAN_ID      = "permanent-solo-0"
	PERMANENT_PLAN_ID = "permanent-owner-99999"
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
	"solo":      {Level: 1, MonthlyBasePrice: 0, YearlyBasePrice: 0, UsersLimit: 1},
	"wholesale": {Level: 2, MonthlyBasePrice: 14900, YearlyBasePrice: 118800, UsersLimit: 3},
	"owner":     {Level: 3, MonthlyBasePrice: PERMANENT, YearlyBasePrice: PERMANENT, UsersLimit: 99999},
}

func LoadConfig() error {
	mode := os.Getenv("MODE")
	if mode == "" {
		helpers.LogError("loadConfig", "MODE is not set in environment")
		return fmt.Errorf("MODE is not set in environment")
	}
	port := os.Getenv("PORT")
	if port == "" {
		helpers.LogError("loadConfig", "PORT is not set in environment")
		return fmt.Errorf("PORT is not set in environment")
	}

	dburl := os.Getenv("DBURL")
	if dburl == "" {
		helpers.LogError("loadConfig", "DBURL is not set in environment")
		return fmt.Errorf("DBURL is not set in environment")
	}

	razorpayKey := os.Getenv("RAZORPAY_API_KEY")
	if razorpayKey == "" {
		helpers.LogError("loadConfig", "RAZORPAY_API_KEY is not set in environment")
		return fmt.Errorf("RAZORPAY_API_KEY is not set in environment")
	}

	razorpaySecret := os.Getenv("RAZORPAY_API_SECRET")
	if razorpaySecret == "" {
		helpers.LogError("loadConfig", "RAZORPAY_API_SECRET is not set in environment")
		return fmt.Errorf("RAZORPAY_API_SECRET is not set in environment")
	}

	googleClientID := os.Getenv("GOOGLE_WEB_CLIENT_ID")
	if googleClientID == "" {
		helpers.LogError("auth.NewHandler", "GOOGLE_WEB_CLIENT_ID is not set in environment")
		return fmt.Errorf("GOOGLE_WEB_CLIENT_ID is not set")
	}

	googleAndroidId := os.Getenv("GOOGLE_ANDROID_ID")
	if googleAndroidId == "" {
		helpers.LogError("auth.NewHandler", "GOOGLE_ANDROID_ID is not set in environment")
		return fmt.Errorf("GOOGLE_ANDROID_ID is not set")
	}

	googleIOSId := os.Getenv("GOOGLE_IOS_ID")
	if googleIOSId == "" {
		helpers.LogError("auth.NewHandler", "GOOGLE_IOS_ID is not set in environment")
		return fmt.Errorf("GOOGLE_IOS_ID is not set")
	}

	firebaseClientId := os.Getenv("FIREBASE_CLIENT_ID")
	if googleIOSId == "" {
		helpers.LogError("auth.NewHandler", "FIREBASE_CLIENT_ID is not set in environment")
		return fmt.Errorf("FIREBASE_CLIENT_ID is not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		helpers.LogError("auth.NewHandler", "JWT_SECRET is not set in environment")
		return fmt.Errorf("JWT_SECRET is not set")
	}

	Configs = configType{
		PORT:                 port,
		DBURL:                dburl,
		RAZORPAY_API_KEY:     razorpayKey,
		RAZORPAY_API_SECRET:  razorpaySecret,
		GOOGLE_WEB_CLIENT_ID: googleClientID,
		GOOGLE_ANDROID_ID:    googleAndroidId,
		GOOGLE_IOS_ID:        googleIOSId,
		FIREBASE_CLIENT_ID:   firebaseClientId,
		JWT_SECRET:           jwtSecret,
	}
	return nil
}
