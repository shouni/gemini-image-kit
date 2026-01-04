package generator

import (
	"context"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// ----------------------------------------------------------------------
// mockImageCore: ImageGeneratorCore のモック
// ----------------------------------------------------------------------

type mockImageCore struct {
	prepareFunc func(ctx context.Context, url string) *genai.Part
	toPartFunc  func(data []byte) *genai.Part
	parseFunc   func(resp *gemini.Response, seed int64) (*ImageOutput, error)
}

func (m *mockImageCore) PrepareImagePart(ctx context.Context, url string) *genai.Part {
	if m.prepareFunc != nil {
		return m.prepareFunc(ctx, url)
	}
	return nil
}

func (m *mockImageCore) ToPart(data []byte) *genai.Part {
	if m.toPartFunc != nil {
		return m.toPartFunc(data)
	}
	return nil
}

func (m *mockImageCore) ParseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error) {
	if m.parseFunc != nil {
		return m.parseFunc(resp, seed)
	}
	return nil, nil
}

// ----------------------------------------------------------------------
// mockAIClient: gemini.GenerativeModel のモック
// ----------------------------------------------------------------------

type mockAIClient struct {
	generateWithPartsFunc func(ctx context.Context, model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error)
	generateContentFunc   func(ctx context.Context, model string, prompt string) (*gemini.Response, error)
}

func (m *mockAIClient) GenerateWithParts(ctx context.Context, model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
	if m.generateWithPartsFunc != nil {
		return m.generateWithPartsFunc(ctx, model, parts, opts)
	}
	return nil, nil
}

func (m *mockAIClient) GenerateContent(ctx context.Context, model string, prompt string) (*gemini.Response, error) {
	if m.generateContentFunc != nil {
		return m.generateContentFunc(ctx, model, prompt)
	}
	return nil, nil
}

func (m *mockAIClient) Close() error { return nil }
