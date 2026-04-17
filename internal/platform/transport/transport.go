package transport

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
}

func writeJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

func ReadJSON(w http.ResponseWriter, r *http.Request, data any) error {
	maxBytes := 1_048_576 // 1MB
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	decode := json.NewDecoder(r.Body)
	return decode.Decode(data)
}

func ReadAndValidJSON(w http.ResponseWriter, r *http.Request, data any) error {
	if err := ReadJSON(w, r, data); err != nil {
		return err
	}
	return validate.Struct(data)
}

func WriteJSONError(w http.ResponseWriter, status int, message string) error {
	type envolop struct {
		Error string `json:"error"`
	}
	return writeJSON(w, status, &envolop{Error: message})
}

func JsonResponse(w http.ResponseWriter, status int, data any) error {
	type envelope struct {
		Data any `json:"data"`
	}
	return writeJSON(w, status, &envelope{Data: data})
}

func ValidateStruct(data any) error {
	return validate.Struct(data)
}
