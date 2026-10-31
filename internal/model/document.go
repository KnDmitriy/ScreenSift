package model

import "time"

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusIndexed    Status = "indexed"
	StatusFailed     Status = "failed"
)

type Document struct {
	ID             int64      `json:"id"`
	Path           string     `json:"path"`
	Size           int64      `json:"size"`
	ModifiedAt     time.Time  `json:"modifiedAt"`
	Status         Status     `json:"status"`
	RecognizedText string     `json:"recognizedText"`
	IndexedAt      *time.Time `json:"indexedAt,omitempty"`
	Error          string     `json:"error,omitempty"`
}
