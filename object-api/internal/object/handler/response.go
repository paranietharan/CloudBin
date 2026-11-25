package handler

import (
	"errors"
	"io"
	"net/http"

	"cloudbin-object-api/internal/object/model"
	"cloudbin-object-api/internal/object/service"
	"cloudbin-shared/response"
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
		response.Error(w, http.StatusConflict, "object changed during upload; retry")
	case errors.Is(err, service.ErrInvalidKey), errors.Is(err, service.ErrInvalidPermission):
		response.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrForbidden):
		response.Error(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrNotFound):
		response.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrReplicationWriteFail):
		response.Error(w, http.StatusBadGateway, err.Error())
	default:
		response.Error(w, http.StatusInternalServerError, "internal server error")
	}
}
