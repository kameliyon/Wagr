package handlers

import (
	"encoding/json"
	"net/http"
)

// RespondJSON writes a JSON response with the given data
func RespondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
