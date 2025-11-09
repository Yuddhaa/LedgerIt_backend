package helpers

import (
	"encoding/json"
	"errors"
	"net/http"
)

func RespondWithJSON(w http.ResponseWriter, code int, payload any) error {
	response, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	// w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	w.Write(response)
	return nil
}

func RespondWithError(w http.ResponseWriter, code int, msg string) error {
	return RespondWithJSON(w, code, map[string]string{"error": msg})
}

func AnyToString(payload any, variable *string) error {
	str, ok := payload.(string)
	if !ok {
		return errors.New("invalid token")
	}
	*variable = str
	return nil
}
