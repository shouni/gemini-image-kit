package generator

import (
	"context"
	"testing"

	"github.com/shouni/gemini-image-kit/ports"
	"google.golang.org/genai"
)

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
	ctx := context.Background()

	// 1. モックのセットアップ
	// Vertex AI モードをシミュレートするモック
	mockAI := &mockAIClient{backend: genai.BackendVertexAI}

	// GeminiImageCore の初期化 (reader や httpClient は nil でもこのテスト範囲なら動きますが、
	// 本来は mockReader 等を渡すのが安全です)
	core, _ := NewGeminiImageCore(mockAI, &mockReader{}, &mockHTTPClient{}, &mockCache{}, 0, true)

	g := &GeminiGenerator{
		core: core,
	}

	tests := []struct {
		name     string
		isVertex bool
		uris     []ports.ImageURI
		verify   func(t *testing.T, parts []*genai.Part)
	}{
		{
			name:     "Vertex AI モードで GCS URI を処理",
			isVertex: true,
			uris: []ports.ImageURI{
				{ReferenceURL: "gs://my-bucket/char.png"},
			},
			verify: func(t *testing.T, parts []*genai.Part) {
				if len(parts) != 1 || parts[0].FileData == nil {
					t.Fatalf("FileData パーツが生成されていません")
				}
				p := parts[0].FileData
				if p.FileURI != "gs://my-bucket/char.png" || p.MIMEType != "image/png" {
					t.Errorf("GCSパスが正しくセットされていません: %+v", p)
				}
			},
		},
		{
			name:     "Gemini API モードで FileAPIURI を優先",
			isVertex: false,
			uris: []ports.ImageURI{
				{
					ReferenceURL: "https://example.com/ignore.jpg",
					FileAPIURI:   "https://generativelanguage.googleapis.com/v1beta/files/abc-123",
				},
			},
			verify: func(t *testing.T, parts []*genai.Part) {
				if len(parts) != 1 || parts[0].FileData == nil {
					t.Fatalf("FileData パーツが生成されていません")
				}
				p := parts[0].FileData
				if p.FileURI != "https://generativelanguage.googleapis.com/v1beta/files/abc-123" {
					t.Errorf("FileAPIURI が優先されていません: %s", p.FileURI)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// テストケースごとに独立したモックとジェネレータを初期化
			backend := genai.BackendGeminiAPI
			if tt.isVertex {
				backend = genai.BackendVertexAI
			}
			mockAI = &mockAIClient{backend: backend}
			core, _ = NewGeminiImageCore(mockAI, &mockReader{}, &mockHTTPClient{}, &mockCache{}, 0, true)
			g = &GeminiGenerator{core: core}

			parts := g.collectImageParts(ctx, tt.uris)
			tt.verify(t, parts)
		})
	}
}
