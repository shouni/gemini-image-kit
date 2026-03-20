package generator

import (
	"testing"

	"github.com/shouni/gemini-image-kit/pkg/ports"
)

// GenerateMangaPanel の構造チェック
func TestGeminiGenerator_GenerateMangaPanel_Structure(t *testing.T) {
	t.Run("FileAPIURIとImageSizeが正しく扱われること", func(t *testing.T) {
		req := ports.ImagePanelRequest{
			GenerationOptions: ports.GenerationOptions{
				Prompt:    "test prompt",
				ImageSize: "2K",
			},
			Image: ports.ImageURI{
				FileAPIURI:   "https://generativelanguage.googleapis.com/v1beta/files/test",
				ReferenceURL: "gs://bucket/ref.png",
			},
		}

		if req.Image.FileAPIURI == "" {
			t.Error("FileAPIURI should be set in req.Image")
		}

		if req.ImageSize != "2K" {
			t.Errorf("ImageSize should be 2K, got %s", req.GenerationOptions.ImageSize)
		}
	})
}

func TestGeminiGenerator_GenerateMangaPage_Structure(t *testing.T) {
	t.Run("複数枚のImageURIが保持されること", func(t *testing.T) {
		req := ports.ImagePageRequest{
			GenerationOptions: ports.GenerationOptions{
				AspectRatio: "16:9",
			},
			Images: []ports.ImageURI{
				{FileAPIURI: "api-1", ReferenceURL: "ref-1"},
				{FileAPIURI: "api-2", ReferenceURL: "ref-2"},
			},
		}

		if len(req.Images) != 2 {
			t.Errorf("expected 2 images, got %d", len(req.Images))
		}

		if req.Images[0].FileAPIURI != "api-1" {
			t.Errorf("first image API URI mismatch: %s", req.Images[0].FileAPIURI)
		}
	})
}
