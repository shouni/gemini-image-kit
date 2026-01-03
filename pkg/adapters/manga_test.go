package adapters

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// 注: mockImageCore と mockAIClient は gemini_adapter_test.go と同じファイル、
// あるいは共通の test_util.go 等に定義されている前提なのだ。

func TestGeminiMangaPageAdapter_GenerateMangaPage(t *testing.T) {
	ctx := context.Background()
	modelName := "imagen-3.0"

	t.Run("Success/ShouldAddMultipleImageURLsToParts", func(t *testing.T) {
		// [修正ポイント] Seed を *int64 で定義
		var seedValue int64 = 999
		req := domain.ImagePageRequest{
			Prompt: "A luxurious manga page",
			ReferenceURLs: []string{
				"http://example.com/chara1.png",
				"http://example.com/chara2.png",
			},
			AspectRatio: "3:4",
			Seed:        &seedValue,
		}

		prepareCallCount := 0
		core := &mockImageCore{
			// [修正ポイント] 引数に context.Context を追加
			prepareFunc: func(ctx context.Context, url string) *genai.Part {
				prepareCallCount++
				return &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("img")}}
			},
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return &ImageOutput{Data: []byte("final-page"), MimeType: "image/png", UsedSeed: seed}, nil
			},
		}

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				// テキスト(1) + 画像(2) = 3パーツ
				if len(parts) != 3 {
					t.Errorf("unexpected number of parts: want 3, got %d", len(parts))
				}
				// [修正ポイント] SDKに *int32 で渡っているか確認
				if opts.Seed == nil || *opts.Seed != int32(seedValue) {
					t.Errorf("seed conversion error: got %v", opts.Seed)
				}
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		adapter := NewGeminiMangaPageAdapter(core, ai, modelName)
		resp, err := adapter.GenerateMangaPage(ctx, req)

		if err != nil {
			t.Fatalf("GenerateMangaPage failed: %v", err)
		}
		if prepareCallCount != 2 {
			t.Errorf("unexpected call count for image preparation: want 2, got %d", prepareCallCount)
		}
		if string(resp.Data) != "final-page" {
			t.Error("unexpected response data")
		}
		if resp.UsedSeed != seedValue {
			t.Errorf("UsedSeed mismatch: want %d, got %d", seedValue, resp.UsedSeed)
		}
	})

	t.Run("Failure/ShouldReturnErrorWhenAIClientFails", func(t *testing.T) {
		req := domain.ImagePageRequest{Prompt: "test failure"}
		expectedErr := errors.New("AI client network error")

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				return nil, expectedErr
			},
		}
		core := &mockImageCore{}

		adapter := NewGeminiMangaPageAdapter(core, ai, modelName)
		_, err := adapter.GenerateMangaPage(ctx, req)

		if err == nil {
			t.Fatal("expected an error but got nil")
		}
		// ラップされている可能性を考慮してメッセージを含んでいるかチェック
		if !strings.Contains(err.Error(), expectedErr.Error()) {
			t.Errorf("expected error containing '%v', but got '%v'", expectedErr, err)
		}
	})

	t.Run("Success/ShouldContinueEvenIfSomeImagesFailToLoad", func(t *testing.T) {
		req := domain.ImagePageRequest{
			Prompt: "Partial failure test",
			ReferenceURLs: []string{
				"http://ok.com/image.png",
				"http://fail.com/bad.png",
			},
		}

		core := &mockImageCore{
			// [修正ポイント] 引数に context.Context を追加
			prepareFunc: func(ctx context.Context, url string) *genai.Part {
				if url == "http://fail.com/bad.png" {
					return nil
				}
				return &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("ok")}}
			},
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return &ImageOutput{Data: []byte("success-anyway")}, nil
			},
		}

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				// 失敗した1枚を除いて、テキスト(1) + 成功画像(1) = 2パーツ
				if len(parts) != 2 {
					t.Errorf("unexpected number of parts: want 2, got %d", len(parts))
				}
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		adapter := NewGeminiMangaPageAdapter(core, ai, modelName)
		resp, err := adapter.GenerateMangaPage(ctx, req)

		if err != nil {
			t.Errorf("GenerateMangaPage should not fail on partial image loading error: %v", err)
		}
		if string(resp.Data) != "success-anyway" {
			t.Error("unexpected response data")
		}
	})

	t.Run("Success/ShouldIgnoreEmptyURLs", func(t *testing.T) {
		req := domain.ImagePageRequest{
			Prompt:        "Empty URL check",
			ReferenceURLs: []string{"", "http://valid.com/img.png", ""},
		}

		prepareCallCount := 0
		core := &mockImageCore{
			prepareFunc: func(ctx context.Context, url string) *genai.Part {
				prepareCallCount++
				return &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("ok")}}
			},
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return &ImageOutput{}, nil
			},
		}

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		adapter := NewGeminiMangaPageAdapter(core, ai, modelName)
		_, err := adapter.GenerateMangaPage(ctx, req)
		if err != nil {
			t.Fatalf("GenerateMangaPage should not return an error, but got: %v", err)
		}

		if prepareCallCount != 1 {
			t.Errorf("empty URLs were not correctly ignored: call count %d", prepareCallCount)
		}
	})
}
