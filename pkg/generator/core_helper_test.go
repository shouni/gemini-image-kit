package generator

import (
	"context"
	"testing"
	"time"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
)

// PrepareImagePart のテスト（キャッシュと変換）
func TestGeminiImageCore_PrepareImagePart(t *testing.T) {
	ctx := context.Background()
	cache := &mockCache{data: make(map[string]any)}
	// mocks_test.go の mockHTTPClient や mockReader を使用
	core := &GeminiImageCore{
		cache:      cache,
		httpClient: &mockHTTPClient{data: []byte("fake-image")},
		reader:     &mockReader{},
	}

	t.Run("キャッシュヒット時はFileDataを返す", func(t *testing.T) {
		rawURL := "https://example.com/img.png"
		fileURI := "https://generativelanguage.googleapis.com/v1beta/files/test-id"
		cache.Set(cacheKeyFileAPIURI+rawURL, fileURI, time.Hour)

		// メソッド名を大文字に変更
		part := core.PrepareImagePart(ctx, rawURL)

		if part == nil || part.FileData == nil {
			t.Fatal("expected FileData part, got nil or other")
		}
		if part.FileData.FileURI != fileURI {
			t.Errorf("got %s, want %s", part.FileData.FileURI, fileURI)
		}
	})

	t.Run("不正なURLはnilを返す(fetchImageData内のIsSafeURLで失敗)", func(t *testing.T) {
		// ローカルホスト等は IsSafeURL で false になる想定
		part := core.PrepareImagePart(ctx, "http://127.0.0.1/evil.png")
		if part != nil {
			t.Error("expected nil for unsafe URL")
		}
	})
}

// ParseToResponse のテスト
func TestGeminiImageCore_ParseToResponse(t *testing.T) {
	core := &GeminiImageCore{}
	seed := int64(999)

	t.Run("正常系: FinishReasonStop", func(t *testing.T) {
		resp := &gemini.Response{
			RawResponse: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						FinishReason: genai.FinishReasonStop,
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

		out, err := core.ParseToResponse(resp, seed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.MimeType != "image/png" || out.UsedSeed != seed {
			t.Errorf("parsed data mismatch: %+v", out)
		}
	})

	t.Run("異常系: FinishReasonSafety によるブロック", func(t *testing.T) {
		resp := &gemini.Response{
			RawResponse: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						FinishReason: genai.FinishReasonSafety,
						Content:      &genai.Content{Parts: []*genai.Part{}},
					},
				},
			},
		}
		_, err := core.ParseToResponse(resp, seed)
		if err == nil {
			t.Error("expected error for Safety block")
		}
		if err != nil && !testing.Short() {
			t.Logf("expected error message: %v", err)
		}
	})

	t.Run("異常系: 画像データなし（テキストのみ）", func(t *testing.T) {
		resp := &gemini.Response{
			RawResponse: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						FinishReason: genai.FinishReasonStop,
						Content:      &genai.Content{Parts: []*genai.Part{{Text: "just text"}}},
					},
				},
			},
		}
		_, err := core.ParseToResponse(resp, seed)
		if err == nil {
			t.Error("expected error for text-only response")
		}
	})

	t.Run("異常系: 空のレスポンス", func(t *testing.T) {
		_, err := core.ParseToResponse(nil, seed)
		if err == nil {
			t.Error("expected error for nil response")
		}
	})
}
