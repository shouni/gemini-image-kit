package domain

import (
	"testing"
)

func TestImageGenerationRequest_Fields(t *testing.T) {
	t.Run("should correctly store FileAPIURI", func(t *testing.T) {
		uri := "https://generativelanguage.googleapis.com/v1beta/files/test-id"
		req := ImageGenerationRequest{
			FileAPIURI: uri,
		}
		if req.FileAPIURI != uri {
			t.Errorf("FileAPIURI is incorrect. want: %s, got: %s", uri, req.FileAPIURI)
		}
	})
}
