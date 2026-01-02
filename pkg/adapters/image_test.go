package adapters

import (
	"context"
	"strings"
	"testing"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// --- Mocks ---

type mockImageCore struct {
	prepareFunc func(url string) *genai.Part
	parseFunc   func(resp *gemini.Response, seed int64) (*ImageOutput, error)
}

func (m *mockImageCore) PrepareImagePart(ctx context.Context, url string) *genai.Part {
	return m.prepareFunc(url)
}
func (m *mockImageCore) ToPart(data []byte) *genai.Part { return nil }
func (m *mockImageCore) ParseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error) {
	return m.parseFunc(resp, seed)
}

type mockAIClient struct {
	generateFunc func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error)
}

func (m *mockAIClient) GenerateWithParts(ctx context.Context, model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
	return m.generateFunc(model, parts, opts)
}

func (m *mockAIClient) GenerateContent(ctx context.Context, model string, prompt string) (*gemini.Response, error) {
	return nil, nil
}

// --- Tests ---

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
				// Verify prompt combination
				if !strings.Contains(parts[0].Text, req.Prompt) || !strings.Contains(parts[0].Text, style) {
					t.Errorf("prompt is not correctly combined: got %s", parts[0].Text)
				}
				// Verify option propagation
				if opts.AspectRatio != req.AspectRatio || opts.Seed == nil || *opts.Seed != seedValue {
					t.Errorf("options are not correctly passed: got %+v", opts)
				}
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		core := &mockImageCore{
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return &ImageOutput{Data: []byte("fake-image"), MimeType: "image/png", UsedSeed: seed}, nil
			},
		}

		adapter, err := NewGeminiImageAdapter(core, ai, modelName, style)
		if err != nil {
			t.Fatalf("failed to create adapter: %v", err)
		}

		resp, err := adapter.GenerateMangaPanel(ctx, req)
		if err != nil {
			t.Fatalf("GenerateMangaPanel should not return error: %v", err)
		}
		if string(resp.Data) != "fake-image" || resp.UsedSeed != int64(seedValue) {
			t.Error("unexpected response data or seed")
		}
	})

	t.Run("Success/ShouldAddImagePartWhenReferenceURLIsProvided", func(t *testing.T) {
		req := domain.ImageGenerationRequest{
			Prompt:       "posing",
			ReferenceURL: "http://example.com/ref.png",
		}

		coreCalled := false
		core := &mockImageCore{
			prepareFunc: func(url string) *genai.Part {
				coreCalled = true
				return &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("ref")}}
			},
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return &ImageOutput{}, nil
			},
		}

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				// Expecting 2 parts: Text + Image
				if len(parts) != 2 {
					t.Errorf("unexpected number of parts: got %d, want 2", len(parts))
				}
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		adapter, err := NewGeminiImageAdapter(core, ai, modelName, style)
		if err != nil {
			t.Fatalf("failed to create adapter: %v", err)
		}

		// Properly check error
		_, err = adapter.GenerateMangaPanel(ctx, req)
		if err != nil {
			t.Fatalf("GenerateMangaPanel should not return error: %v", err)
		}

		if !coreCalled {
			t.Error("PrepareImagePart was not called")
		}
	})
}
