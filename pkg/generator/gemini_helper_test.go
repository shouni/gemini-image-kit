package generator

import (
	"context"
	"testing"

	"github.com/shouni/gemini-image-kit/pkg/domain"
)

func TestGuessMIMEType(t *testing.T) {
	g := &GeminiGenerator{}

	tests := []struct {
		uri      string
		expected string
	}{
		{"gs://bucket/image.jpg", "image/jpeg"},
		{"https://example.com/photo.JPEG", "image/jpeg"},
		{"path/to/icon.png", "image/png"},
		{"image.webp", "image/webp"},
		{"document.pdf", "image/jpeg"}, // 未対応拡張子はフォールバック
		{"no_extension", "image/jpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			got := g.guessMIMEType(tt.uri)
			if got != tt.expected {
				t.Errorf("guessMIMEType(%q) = %q; want %q", tt.uri, got, tt.expected)
			}
		})
	}
}

func TestBuildFinalPrompt(t *testing.T) {
	// 実装側の negativePromptSeparator と一致させる
	const sep = negativePromptSeparator

	tests := []struct {
		name     string
		prompt   string
		negative string
		expected string
	}{
		{
			name:     "プロンプトのみ",
			prompt:   "a cute cat",
			negative: "",
			expected: "a cute cat",
		},
		{
			name:     "両方あり",
			prompt:   "a cute cat",
			negative: "blurry, low quality",
			expected: "a cute cat" + sep + "blurry, low quality",
		},
		{
			name:     "空の入力",
			prompt:   "  ",
			negative: "",
			expected: "",
		},
		{
			name:     "トリミング確認",
			prompt:   "  spaced prompt  ",
			negative: "  spaced negative  ",
			expected: "spaced prompt" + sep + "spaced negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFinalPrompt(tt.prompt, tt.negative)
			if got != tt.expected {
				t.Errorf("buildFinalPrompt() = %q; want %q", got, tt.expected)
			}
		})
	}
}

func TestCollectImageParts(t *testing.T) {
	g := &GeminiGenerator{}
	ctx := context.Background()

	uris := []domain.ImageURI{
		{
			ReferenceURL: "gs://my-bucket/char.png",
		},
		{
			FileAPIURI: "https://generativelanguage.googleapis.com/v1beta/files/abc-123",
		},
	}

	parts := g.collectImageParts(ctx, uris)

	if len(parts) != 2 {
		t.Fatalf("期待されるパーツ数は 2 ですが、%d でした", len(parts))
	}

	// 1. GCSの検証
	p1 := parts[0].FileData
	if p1.FileURI != "gs://my-bucket/char.png" || p1.MIMEType != "image/png" {
		t.Errorf("GCSパーツのデータが不正です: %+v", p1)
	}

	// 2. File APIの検証
	p2 := parts[1].FileData
	if p2.FileURI != uris[1].FileAPIURI || p2.MIMEType != "image/jpeg" {
		t.Errorf("File APIパーツのデータが不正です: %+v", p2)
	}
}
