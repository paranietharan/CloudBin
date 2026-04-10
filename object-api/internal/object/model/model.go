package model

import "time"

type ObjectRecord struct {
	ID          string
	OwnerUserID string
	ObjectKey   string
	ContentType string
	SizeBytes   int64
	ETag        string
	Permission  string
	Visibility  string
	PrimaryNode string
	ReplicaNode string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
