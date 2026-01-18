package generator

import (
	"context"
	"io"
	"time"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
)

// --- Mocks ---

type mockAIClient struct {
	uploadCalled bool
	deleteCalled bool
	lastFileName string
}

func (m *mockAIClient) UploadFile(ctx context.Context, data []byte, mimeType, displayName string) (string, string, error) {
	m.uploadCalled = true
	return "https://gemini.api/files/new-file-id", "files/new-file-id", nil
}

func (m *mockAIClient) DeleteFile(ctx context.Context, name string) error {
	m.deleteCalled = true
	m.lastFileName = name
	return nil
}

func (m *mockAIClient) GenerateContent(ctx context.Context, model string, prompt string) (*gemini.Response, error) {
	return nil, nil
}

func (m *mockAIClient) GenerateWithParts(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*gemini.Response, error) {
	return &gemini.Response{
		RawResponse: &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{{
				Content: &genai.Content{
					Parts: []*genai.Part{{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("fake")}}},
				},
			}},
		},
	}, nil
}

func (m *mockAIClient) GetFile(ctx context.Context, name string) (*genai.File, error) {
	return nil, nil
}

// mockReader の修正: List メソッドをコールバック形式に変更
type mockReader struct{}

func (m *mockReader) Open(ctx context.Context, uri string) (io.ReadCloser, error) {
	return nil, nil
}

// エラー内容に合わせ、シグネチャを修正しました
func (m *mockReader) List(ctx context.Context, uri string, fn func(string) error) error {
	return nil
}

type mockHTTPClient struct {
	data []byte
	err  error
}

func (m *mockHTTPClient) FetchBytes(ctx context.Context, url string) ([]byte, error) {
	return m.data, m.err
}

type mockCache struct {
	data map[string]any
}

func (m *mockCache) Get(key string) (any, bool) {
	val, ok := m.data[key]
	return val, ok
}

func (m *mockCache) Set(key string, value any, d time.Duration) {
	m.data[key] = value
}
