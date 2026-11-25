package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
)

func allowMethod(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	if slices.Contains(methods, r.Method) {
		return true
	}
	respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	return false
}

type keyRequest struct {
	ObjectKey string `json:"object_key"`
	OwnerID   string `json:"owner_id"`
}

func readObjectKey(r *http.Request) string {
	if key := strings.TrimSpace(r.URL.Query().Get("object_key")); key != "" {
		return key
	}
	if key := strings.TrimSpace(r.URL.Query().Get("key")); key != "" {
		return key
	}
	if headerKey := strings.TrimSpace(r.Header.Get("X-Object-Key")); headerKey != "" {
		return headerKey
	}

	return strings.TrimSpace(readKeyRequest(r).ObjectKey)
}

func readOwnerID(r *http.Request) string {
	if ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id")); ownerID != "" {
		return ownerID
	}
	return strings.TrimSpace(readKeyRequest(r).OwnerID)
}

func readKeyRequest(r *http.Request) keyRequest {
	if r.Body == nil || !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		return keyRequest{}
	}
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		return keyRequest{}
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(buf))

	var req keyRequest
	if err := json.Unmarshal(buf, &req); err != nil {
		return keyRequest{}
	}
	return req
}
