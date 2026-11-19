package model

import (
	"strings"
	"time"
)

type ObjectRecord struct {
	ID          string
	OwnerUserID string
	ObjectKey   string
	ContentType string
	SizeBytes   int64
	Nodes       []string `json:"-"`
	StorageKey  string   `json:"-"`
	ETag        string
	Permission  string
	Visibility  string
	PrimaryNode string
	ReplicaNode string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r ObjectRecord) BlobKey() string {
	if r.StorageKey != "" {
		return r.StorageKey
	}
	return strings.TrimSpace(r.OwnerUserID) + "/" + strings.TrimPrefix(strings.TrimSpace(r.ObjectKey), "/")
}
