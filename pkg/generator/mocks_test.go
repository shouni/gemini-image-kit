package generator

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// ----------------------------------------------------------------------
// mockReader: remoteio.InputReader のモック
// ----------------------------------------------------------------------

type mockReader struct {
	fetchFunc func(ctx context.Context, url string) ([]byte, error)
}

func (m *mockReader) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	if m.fetchFunc != nil {
		data, err := m.fetchFunc(ctx, name)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return nil, io.EOF
}

func (m *mockReader) FetchBytes(ctx context.Context, url string) ([]byte, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, url)
	}
	return nil, io.EOF
}

// ----------------------------------------------------------------------
// mockHTTPClient: HTTPClient のモック
// ----------------------------------------------------------------------

type mockHTTPClient struct {
	fetchFunc func(ctx context.Context, url string) ([]byte, error)
}

func (m *mockHTTPClient) FetchBytes(ctx context.Context, url string) ([]byte, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, url)
	}
	return nil, nil
}

// ----------------------------------------------------------------------
// mockCache: ImageCacher のモック
// ----------------------------------------------------------------------

type mockCache struct {
	data map[string]any
}

func (m *mockCache) Get(key string) (any, bool) {
	if m.data == nil {
		return nil, false
	}
	v, ok := m.data[key]
	return v, ok
}

func (m *mockCache) Set(key string, value any, d time.Duration) {
	if m.data == nil {
		m.data = make(map[string]any)
	}
	m.data[key] = value
}

// ----------------------------------------------------------------------
// mockImageCore: ImageGeneratorCore のモック (上位層向け)
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
