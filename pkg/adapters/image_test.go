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
		seedValue := int32(1234)
		req := domain.ImageGenerationRequest{
			Prompt:      "zundamon running",
			AspectRatio: "16:9",
			Seed:        &seedValue,
		}

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				if !strings.Contains(parts[0].Text, req.Prompt) || !strings.Contains(parts[0].Text, style) {
					t.Errorf("prompt is not correctly combined: got %s", parts[0].Text)
				}
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		core := &mockImageCore{
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
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

		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error '%v', but got '%v'", expectedErr, err)
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
