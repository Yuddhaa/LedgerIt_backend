package helpers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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

// isUniqueViolation is a helper function to check for a PostgreSQL unique_violation error (code 23505).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// StringToUUID converts given string to pgtype.UUID
func StringToUUID(w http.ResponseWriter, r *http.Request, variable, uuidStr string) (pgtype.UUID, bool) {
	uuidBytes, err := uuid.Parse(uuidStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Bad Request: "+variable)
		LogError("StringToUUID", "error in converting "+variable+" to uuid", "error", err.Error(), "string", uuidStr)
		return pgtype.UUID{}, false
	}
	return pgtype.UUID{
		Bytes: uuidBytes,
		Valid: uuidBytes != uuid.Nil,
	}, true
}
