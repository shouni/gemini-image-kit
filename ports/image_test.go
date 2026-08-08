package ports

import (
	"testing"

	"github.com/shouni/go-gemini-client/gemini"
)

// TestGenerationOptionsPromotesGeminiFields は、埋め込んだ gemini.GenerateOptions の
// フィールドが昇格で読み書きできることを確認します。この昇格が利用側コードの
// 互換性（req.ImageSize のようなアクセス）を支えています。
func TestGenerationOptionsPromotesGeminiFields(t *testing.T) {
	req := ImageRequest{
		GenerationOptions: GenerationOptions{
			Model:  "gemini-test",
			Prompt: "a cat",
			GenerateOptions: gemini.GenerateOptions{
				ImageSize:   "2K",
				AspectRatio: "16:9",
			},
		},
		Images: []ImageURI{{ReferenceURL: "gs://bucket/ref.png"}},
	}

	if req.ImageSize != "2K" {
		t.Errorf("promoted ImageSize = %q, want 2K", req.ImageSize)
	}
	if req.AspectRatio != "16:9" {
		t.Errorf("promoted AspectRatio = %q, want 16:9", req.AspectRatio)
	}

	req.ImageSize = "1K" // 昇格フィールドへの書き込み
	if got := req.GenerateOptions; got.ImageSize != "1K" {
		t.Errorf("write through promotion failed: %q", got.ImageSize)
	}
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
