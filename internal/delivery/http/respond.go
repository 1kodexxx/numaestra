package apphttp

import (
	"encoding/json"
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// respondError пишет JSON-ошибку и добавляет request_id из контекста запроса.
// Единый формат для всех хендлеров: {"error":"...", "request_id":"..."}.
func respondError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	body := map[string]string{"error": msg}
	if reqID := chimiddleware.GetReqID(r.Context()); reqID != "" {
		body["request_id"] = reqID
	}
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}

// respondJSON сериализует data в JSON с заданным HTTP-статусом.
func respondJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data) //nolint:errcheck
}
