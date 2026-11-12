package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

func respondJSON(w http.ResponseWriter, status int, payload any) {
	success := status >= http.StatusOK && status < http.StatusMultipleChoices
	wrapped := payload
	switch v := payload.(type) {
	case map[string]any:
		if _, ok := v["success"]; !ok {
			v["success"] = success
		}
		wrapped = v
	case map[string]string:
		m := make(map[string]any, len(v)+1)
		for k, val := range v {
			m[k] = val
		}
		m["success"] = success
		wrapped = m
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(wrapped)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message, "error_code": errorCodeFromStatus(status)})
}

func errorCodeFromStatus(status int) string {
	code := strings.ToLower(http.StatusText(status))
	code = strings.ReplaceAll(code, " ", "_")
	if code == "" {
		return "internal_server_error"
	}
	return code
}
