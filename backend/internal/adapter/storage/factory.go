package storage

import (
	"net/http"
	"time"

	answerocrapp "mathstudy/backend/internal/application/answerocr"
	uploadapp "mathstudy/backend/internal/application/upload"
)

// UploadBackend supports both upload writes and trusted answer-image reads.
type UploadBackend interface {
	uploadapp.Storage
	answerocrapp.ImageLoader
}

func defaultTimeout(client *http.Client) *http.Client {
	if client == nil {
		return &http.Client{Timeout: 5 * time.Minute}
	}
	if client.Timeout == 0 {
		copy := *client
		copy.Timeout = 5 * time.Minute
		return &copy
	}
	return client
}
