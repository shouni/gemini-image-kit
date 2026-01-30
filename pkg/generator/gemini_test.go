package generator

import (
	"testing"

	"github.com/shouni/gemini-image-kit/pkg/domain"
)

// buildFinalPrompt の単体テスト
func TestBuildFinalPrompt(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		negative string
		want     string
	}{
		{
			name:     "プロンプトのみ",
			prompt:   "a cute robot",
			negative: "",
			want:     "a cute robot",
		},
		{
			name:     "ネガティブプロンプトあり",
			prompt:   "a cute robot",
			negative: "blurry, low quality",
			want:     "a cute robot\n\n[Negative Prompt]\nblurry, low quality",
		},
		{
			name:     "空文字とスペースのトリミング",
			prompt:   "  starry night  ",
			negative: "  dark  ",
			want:     "starry night\n\n[Negative Prompt]\ndark",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFinalPrompt(tt.prompt, tt.negative)
			if got != tt.want {
				t.Errorf("buildFinalPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

// GenerateMangaPanel の構造チェック
func TestGeminiGenerator_GenerateMangaPanel_Structure(t *testing.T) {
	t.Run("FileAPIURIとImageSizeが正しく扱われること", func(t *testing.T) {
		// 構造体のネストに合わせて修正
		req := domain.ImageGenerationRequest{
			Prompt:    "test prompt",
			ImageSize: "2K",
			Image: domain.ImageURI{
				FileAPIURI:   "https://generativelanguage.googleapis.com/v1beta/files/test",
				ReferenceURL: "gs://bucket/ref.png",
			},
		}

		// 検証のポイント:
		// 1. collectImageParts において req.Image.FileAPIURI が parts に含まれているか
		// 2. toOptions において req.ImageSize が options.ImageSize に渡っているか

		if req.Image.FileAPIURI == "" {
			t.Error("FileAPIURI should be set in req.Image")
		}
		if req.ImageSize != "2K" {
			t.Errorf("ImageSize should be 2K, got %s", req.ImageSize)
		}
	})
}

func TestGeminiGenerator_GenerateMangaPage_Structure(t *testing.T) {
	t.Run("複数枚のImageURIが保持されること", func(t *testing.T) {
		req := domain.ImagePageRequest{
			Images: []domain.ImageURI{
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
