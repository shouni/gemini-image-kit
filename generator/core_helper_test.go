package generator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
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
		part, err := core.PrepareImagePart(ctx, rawURL)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if part == nil || part.FileData == nil {
			t.Fatal("expected FileData part, got nil or other")
		}
		if part.FileData.FileURI != fileURI {
			t.Errorf("got %s, want %s", part.FileData.FileURI, fileURI)
		}
	})

	t.Run("画像として扱えないデータはエラーを返す", func(t *testing.T) {
		cache.Clear()
		part, err := core.PrepareImagePart(ctx, "https://example.com/not-image.png")
		if err == nil {
			t.Fatal("expected error for invalid image data")
		}
		if part != nil {
			t.Error("expected nil part for invalid image data")
		}
	})

	t.Run("取得失敗はエラーを返す", func(t *testing.T) {
		cache.Clear()
		fetchErr := errors.New("network down")
		core := &GeminiImageCore{
			cache:      cache,
			httpClient: &mockHTTPClient{err: fetchErr},
			reader:     &mockReader{},
		}

		part, err := core.PrepareImagePart(ctx, "https://example.com/image.png")
		if err == nil {
			t.Fatal("expected fetch error")
		}
		if part != nil {
			t.Error("expected nil part for fetch error")
		}
	})
}

// parseToResponse のテスト
func TestGeminiImageCore_parseToResponse(t *testing.T) {
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

		out, err := core.parseToResponse(resp, seed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.MimeType != "image/png" || out.UsedSeed != seed {
			t.Errorf("parsed data mismatch: %+v", out)
		}
	})

	// NOTE: FinishReason の検証（安全ブロック等）は go-gemini-client 側が行うため、
	// parseToResponse では画像データの有無のみを検証します。
	t.Run("異常系: Parts が空（ブロック等で画像が返らなかった場合）", func(t *testing.T) {
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
		_, err := core.parseToResponse(resp, seed)
		if err == nil {
			t.Error("expected error when no image data is present")
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
		_, err := core.parseToResponse(resp, seed)
		if err == nil {
			t.Error("expected error for text-only response")
		}
	})

	t.Run("異常系: 空のレスポンス", func(t *testing.T) {
		_, err := core.parseToResponse(nil, seed)
		if err == nil {
			t.Error("expected error for nil response")
		}
	})
}
