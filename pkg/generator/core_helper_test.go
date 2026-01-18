package generator

import (
	"context"
	"testing"
	"time"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
)

// prepareImagePart のテスト（キャッシュと変換）
func TestGeminiImageCore_PrepareImagePart(t *testing.T) {
	ctx := context.Background()
	cache := &mockCache{data: make(map[string]any)}
	core := &GeminiImageCore{cache: cache}

	t.Run("キャッシュヒット時はFileDataを返す", func(t *testing.T) {
		rawURL := "https://example.com/img.png"
		fileURI := "https://generativelanguage.googleapis.com/v1beta/files/test-id"
		cache.Set(cacheKeyFileAPIURI+rawURL, fileURI, time.Hour)

		part := core.prepareImagePart(ctx, rawURL)

		if part == nil || part.FileData == nil {
			t.Fatal("expected FileData part, got nil or other")
		}
		if part.FileData.FileURI != fileURI {
			t.Errorf("got %s, want %s", part.FileData.FileURI, fileURI)
		}
	})

	t.Run("不正なURLはnilを返す(fetchImageData内のIsSafeURLで失敗)", func(t *testing.T) {
		part := core.prepareImagePart(ctx, "http://127.0.0.1/evil.png")
		if part != nil {
			t.Error("expected nil for unsafe URL")
		}
	})
}

// parseToResponse のテスト
func TestGeminiImageCore_ParseToResponse(t *testing.T) {
	core := &GeminiImageCore{}
	seed := int64(999)

	t.Run("正常系", func(t *testing.T) {
		resp := &gemini.Response{
			RawResponse: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{
									InlineData: &genai.Blob{
										MIMEType: "image/png",
										Data:     []byte("png-data"),
									},
								},
							},
						},
					},
				},
			},
		}

		out, err := core.parseToResponse(resp, seed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.MimeType != "image/png" || out.UsedSeed != seed {
			t.Errorf("parsed data mismatch: %+v", out)
		}
	})

	t.Run("異常系: 画像データなし", func(t *testing.T) {
		resp := &gemini.Response{
			RawResponse: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{Content: &genai.Content{Parts: []*genai.Part{{Text: "just text"}}}},
				},
			},
		}
		_, err := core.parseToResponse(resp, seed)
		if err == nil {
			t.Error("expected error for text-only response")
		}
	})
}
