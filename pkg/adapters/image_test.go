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

func TestGeminiImageAdapter_GenerateMangaPanel(t *testing.T) {
	ctx := context.Background()
	modelName := "imagen-3.0"
	style := "anime style, high quality"

	t.Run("Success/ShouldPassPromptAndOptionsToAIClientCorrectly", func(t *testing.T) {
		// [修正ポイント] Seed を *int64 に合わせるのだ
		var seedValue int64 = 1234
		req := domain.ImageGenerationRequest{
			Prompt:      "zundamon running",
			AspectRatio: "16:9",
			Seed:        &seedValue,
		}

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				// プロンプト結合のチェック
				if !strings.Contains(parts[0].Text, req.Prompt) || !strings.Contains(parts[0].Text, style) {
					t.Errorf("prompt is not correctly combined: got %s", parts[0].Text)
				}
				// [修正ポイント] SDKに渡る際は *int32 に変換されているかチェックするのだ
				if opts.Seed == nil || *opts.Seed != int32(seedValue) {
					t.Errorf("seed was not correctly converted to int32: got %v", opts.Seed)
				}
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		core := &mockImageCore{
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				// [修正ポイント] パース関数に渡る seed が int64 のままであることを確認
				return &ImageOutput{Data: []byte("fake-image"), MimeType: "image/png", UsedSeed: seed}, nil
			},
		}

		adapter, _ := NewGeminiImageAdapter(core, ai, modelName, style)
		resp, err := adapter.GenerateMangaPanel(ctx, req)

		if err != nil {
			t.Fatalf("GenerateMangaPanel should not return error: %v", err)
		}
		if string(resp.Data) != "fake-image" {
			t.Error("unexpected response data")
		}
		if resp.UsedSeed != seedValue {
			t.Errorf("expected seed %d, got %d", seedValue, resp.UsedSeed)
		}
	})

	t.Run("Failure/ShouldReturnErrorWhenAIClientFails", func(t *testing.T) {
		req := domain.ImageGenerationRequest{Prompt: "test failure"}
		expectedErr := errors.New("AI client error")

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				return nil, expectedErr
			},
		}
		core := &mockImageCore{}

		adapter, _ := NewGeminiImageAdapter(core, ai, modelName, style)
		_, err := adapter.GenerateMangaPanel(ctx, req)

		if err == nil || !strings.Contains(err.Error(), expectedErr.Error()) {
			t.Errorf("expected error containing '%v', but got '%v'", expectedErr, err)
		}
	})

	t.Run("Failure/ShouldReturnErrorWhenParsingFails", func(t *testing.T) {
		req := domain.ImageGenerationRequest{Prompt: "test parse failure"}
		expectedErr := errors.New("parse error")

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}
		core := &mockImageCore{
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return nil, expectedErr
			},
		}

		adapter, _ := NewGeminiImageAdapter(core, ai, modelName, style)
		_, err := adapter.GenerateMangaPanel(ctx, req)

		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error '%v', but got '%v'", expectedErr, err)
		}
	})
}
