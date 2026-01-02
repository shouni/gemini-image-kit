package adapters

import (
	"context"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// mockImageCore implements ImageGeneratorCore for testing.
type mockImageCore struct {
	prepareFunc func(url string) *genai.Part
	parseFunc   func(resp *gemini.Response, seed int64) (*ImageOutput, error)
}

func (m *mockImageCore) PrepareImagePart(ctx context.Context, url string) *genai.Part {
	if m.prepareFunc != nil {
		return m.prepareFunc(url)
	}
	return nil
}
func (m *mockImageCore) ToPart(data []byte) *genai.Part { return nil }
func (m *mockImageCore) ParseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error) {
	if m.parseFunc != nil {
		return m.parseFunc(resp, seed)
	}
	return nil, nil
}

// mockAIClient implements gemini.GenerativeModel for testing.
type mockAIClient struct {
	generateFunc func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error)
}

func (m *mockAIClient) GenerateWithParts(ctx context.Context, model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
	return m.generateFunc(model, parts, opts)
}

func (m *mockAIClient) GenerateContent(ctx context.Context, model string, prompt string) (*gemini.Response, error) {
	return nil, nil
}
