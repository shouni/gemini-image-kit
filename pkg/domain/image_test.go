package domain

import (
	"testing"
)

func TestImageGenerationRequest_FieldValidation(t *testing.T) {
	t.Run("should correctly store both ReferenceURL and FileAPIURI", func(t *testing.T) {
		refURL := "gs://my-bucket/char.png"
		apiURI := "https://generativelanguage.googleapis.com/v1beta/files/file-123"

		req := ImageGenerationRequest{
			Prompt:       "anime style character",
			ReferenceURL: refURL,
			FileAPIURI:   apiURI,
		}

		if req.ReferenceURL != refURL {
			t.Errorf("ReferenceURL mismatch. want: %s, got: %s", refURL, req.ReferenceURL)
		}
		if req.FileAPIURI != apiURI {
			t.Errorf("FileAPIURI mismatch. want: %s, got: %s", apiURI, req.FileAPIURI)
		}
	})

	t.Run("should handle nil seed as random", func(t *testing.T) {
		req := ImageGenerationRequest{
			Prompt: "random seed test",
			Seed:   nil,
		}

		if req.Seed != nil {
			t.Error("Seed should be nil for random generation")
		}
	})

	t.Run("should correctly store a specified int64 seed value", func(t *testing.T) {
		var val int64 = 9223372036854775807 // MaxInt64
		req := ImageGenerationRequest{
			Seed: &val,
		}

		if req.Seed == nil {
			t.Fatalf("Seed pointer should not be nil")
		}
		if *req.Seed != val {
			t.Errorf("Seed value precision lost. want: %d, got: %d", val, *req.Seed)
		}
	})
}

func TestImagePageRequest_MultiResource(t *testing.T) {
	t.Run("should handle multiple ReferenceURLs and FileAPIURIs", func(t *testing.T) {
		refs := []string{"gs://b/1.png", "gs://b/2.png"}
		apis := []string{"https://api/f1", "https://api/f2"}

		req := ImagePageRequest{
			ReferenceURLs: refs,
			FileAPIURIs:   apis,
		}

		if len(req.ReferenceURLs) != 2 || len(req.FileAPIURIs) != 2 {
			t.Errorf("Slice lengths mismatch. refs: %d, apis: %d", len(req.ReferenceURLs), len(req.FileAPIURIs))
		}

		if req.FileAPIURIs[0] != apis[0] {
			t.Errorf("First FileAPIURI mismatch. want: %s, got: %s", apis[0], req.FileAPIURIs[0])
		}
	})
}

func TestImageResponse_TypeConsistency(t *testing.T) {
	t.Run("should maintain int64 precision for the used seed", func(t *testing.T) {
		var largeSeed int64 = 9223372036854775807
		resp := ImageResponse{
			Data:     []byte{0xFF, 0xD8},
			MimeType: "image/jpeg",
			UsedSeed: largeSeed,
		}

		if resp.UsedSeed != largeSeed {
			t.Errorf("UsedSeed value precision lost. want: %d, got: %d", largeSeed, resp.UsedSeed)
		}
	})
}
