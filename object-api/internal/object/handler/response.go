package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"cloudbin-object-api/internal/object/model"
	"cloudbin-object-api/internal/object/service"
)

func setObjectHeaders(w http.ResponseWriter, rec model.ObjectRecord) {
	if rec.ContentType != "" {
		w.Header().Set("Content-Type", rec.ContentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if rec.ETag != "" {
		w.Header().Set("ETag", rec.ETag)
	}
}

func writeDownload(w http.ResponseWriter, rec model.ObjectRecord, stream io.ReadCloser, err error) {
	if err != nil {
		respondServiceError(w, err)
		return
	}
	defer stream.Close()

	setObjectHeaders(w, rec)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, stream)
}

func respondServiceError(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, service.ErrConflict):
		respondError(w, http.StatusConflict, "object changed during upload; retry")
	case errors.Is(err, service.ErrInvalidKey), errors.Is(err, service.ErrInvalidPermission):
		respondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrForbidden):
		respondError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrReplicationWriteFail):
		respondError(w, http.StatusBadGateway, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "internal server error")
	}
}

func respondJSON(w http.ResponseWriter, status int, payload map[string]any) {
	payload["success"] = status >= http.StatusOK && status < http.StatusMultipleChoices
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]any{"error": message, "error_code": errorCodeFromStatus(status)})
}

func errorCodeFromStatus(status int) string {
	code := strings.ToLower(http.StatusText(status))
	code = strings.ReplaceAll(code, " ", "_")
	if code == "" {
		return "internal_server_error"
	}
	return code
}
