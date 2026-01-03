package adapters

import (
	"context"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// mockImageCore は ImageGeneratorCore インターフェースのテスト用モックなのだ。
type mockImageCore struct {
	// 引数に ctx を追加してインターフェースと一致させるのが安全なのだ
	prepareFunc func(ctx context.Context, url string) *genai.Part
	parseFunc   func(resp *gemini.Response, seed int64) (*ImageOutput, error)
}

func (m *mockImageCore) PrepareImagePart(ctx context.Context, url string) *genai.Part {
	if m.prepareFunc != nil {
		return m.prepareFunc(ctx, url)
	}
	return nil
}

// 必要に応じて実装を追加できるよう nil 以外を返さない形にしておくのだ
func (m *mockImageCore) ToPart(data []byte) *genai.Part { return nil }

func (m *mockImageCore) ParseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error) {
	if m.parseFunc != nil {
		return m.parseFunc(resp, seed)
	}
	return nil, nil
}

// mockAIClient は gemini.GenerativeModel のテスト用モックなのだ。
type mockAIClient struct {
	// 他のメソッド（GenerateContent等）を埋め込みで解決するために interface を持たせると便利なのだ
	gemini.GenerativeModel
	generateFunc func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error)
}

func (m *mockAIClient) GenerateWithParts(ctx context.Context, model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
	if m.generateFunc != nil {
		return m.generateFunc(model, parts, opts)
	}
	return nil, nil
}

// インターフェースを満たすために空の実装を置いておくのだ
func (m *mockAIClient) GenerateContent(ctx context.Context, model string, prompt string) (*gemini.Response, error) {
	return nil, nil
}
