package domain

import (
	"testing"
)

func TestImageGenerationRequest_Fields(t *testing.T) {
	t.Run("should correctly store ImageURI and Options", func(t *testing.T) {
		fileAPI := "https://generativelanguage.googleapis.com/v1beta/files/test-id"
		refURL := "gs://my-bucket/character.png"
		size := "2K"

		req := ImagePanelRequest{
			Image: ImageURI{
				FileAPIURI:   fileAPI,
				ReferenceURL: refURL,
			},
			Options: GenerationOptions{
				ImageSize: size,
			},
		}

		// ImageURI 経由の確認
		if req.Image.FileAPIURI != fileAPI {
			t.Errorf("FileAPIURI is incorrect. want: %s, got: %s", fileAPI, req.Image.FileAPIURI)
		}
		if req.Image.ReferenceURL != refURL {
			t.Errorf("ReferenceURL is incorrect. want: %s, got: %s", refURL, req.Image.ReferenceURL)
		}

		// Options 経由の確認
		if req.Options.ImageSize != size {
			t.Errorf("ImageSize is incorrect. want: %s, got: %s", size, req.Options.ImageSize)
		}
	})
}

func TestImagePageRequest_Fields(t *testing.T) {
	t.Run("should correctly store multiple ImageURIs", func(t *testing.T) {
		uris := []ImageURI{
			{ReferenceURL: "url1", FileAPIURI: "api1"},
			{ReferenceURL: "url2", FileAPIURI: "api2"},
		}

		req := ImagePageRequest{
			Images: uris,
		}

		if len(req.Images) != 2 {
			t.Fatalf("Images length is incorrect. want: 2, got: %d", len(req.Images))
		}

		if req.Images[1].FileAPIURI != "api2" {
			t.Errorf("Second Image FileAPIURI is incorrect. want: api2, got: %s", req.Images[1].FileAPIURI)
		}
	})
}
