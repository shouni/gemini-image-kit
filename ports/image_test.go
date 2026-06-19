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
			t.Errorf("ImageSize is incorrect. want: %s, got: %s", size, req.GenerationOptions.ImageSize)
		}
	})
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

func TestEditImageRequest_Fields(t *testing.T) {
	t.Run("should correctly store edit image fields", func(t *testing.T) {
		seed := int64(123)
		bbox := &BoundingBox{X: 10, Y: 20, Width: 100, Height: 80}

		req := EditImageRequest{
			Model:      "imagen-edit",
			EditPrompt: "replace the object",
			Image:      ImageURI{ReferenceURL: "gs://bucket/source.png"},
			Mask:       ImageURI{ReferenceURL: "gs://bucket/mask.png"},
			TargetBBox: bbox,
			Seed:       &seed,
		}

		if req.Model != "imagen-edit" {
			t.Errorf("Model is incorrect. want: imagen-edit, got: %s", req.Model)
		}
		if req.Image.ReferenceURL != "gs://bucket/source.png" {
			t.Errorf("Image ReferenceURL is incorrect. got: %s", req.Image.ReferenceURL)
		}
		if req.Mask.ReferenceURL != "gs://bucket/mask.png" {
			t.Errorf("Mask ReferenceURL is incorrect. got: %s", req.Mask.ReferenceURL)
		}
		if req.EditPrompt != "replace the object" {
			t.Errorf("EditPrompt is incorrect. got: %s", req.EditPrompt)
		}
		if req.TargetBBox != bbox {
			t.Fatal("TargetBBox should be stored as provided")
		}
		if req.Seed == nil || *req.Seed != seed {
			t.Fatalf("Seed is incorrect. got: %v", req.Seed)
		}
	})
}
