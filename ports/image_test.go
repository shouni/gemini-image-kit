package ports

import (
	"testing"
)

func TestSingleImageRequest_Fields(t *testing.T) {
	t.Run("should correctly store ImageURI and GenerationOptions", func(t *testing.T) {
		fileAPI := "https://generativelanguage.googleapis.com/v1beta/files/test-id"
		refURL := "gs://my-bucket/character.png"
		size := "2K"

		req := SingleImageRequest{
			Image: ImageURI{
				FileAPIURI:   fileAPI,
				ReferenceURL: refURL,
			},
			GenerationOptions: GenerationOptions{
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

		if req.ImageSize != size {
			t.Errorf("ImageSize is incorrect. want: %s, got: %s", size, req.ImageSize)
		}
	})
}

func TestImageURI_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		uri  ImageURI
		want bool
	}{
		{name: "empty", uri: ImageURI{}, want: true},
		{name: "reference URL", uri: ImageURI{ReferenceURL: "https://example.com/image.png"}, want: false},
		{name: "file API URI", uri: ImageURI{FileAPIURI: "https://generativelanguage.googleapis.com/v1beta/files/test"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.uri.IsEmpty(); got != tt.want {
				t.Fatalf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageFusionRequest_Fields(t *testing.T) {
	t.Run("should correctly store multiple ImageURIs", func(t *testing.T) {
		uris := []ImageURI{
			{ReferenceURL: "url1", FileAPIURI: "api1"},
			{ReferenceURL: "url2", FileAPIURI: "api2"},
		}

		req := ImageFusionRequest{
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
