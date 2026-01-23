package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
)

// --- AI Client Mock ---

type mockAIClient struct {
	uploadCalled bool
	deleteCalled bool
	lastFileName string
}

func (m *mockAIClient) UploadFile(ctx context.Context, data []byte, mimeType, displayName string) (string, string, error) {
	m.uploadCalled = true
	// テスト期待値と一致させる URI
	return "https://generativelanguage.googleapis.com/v1beta/files/mock-id", "files/mock-id", nil
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
				FinishReason: genai.FinishReasonStop,
				Content: &genai.Content{
					Parts: []*genai.Part{
						{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("fake-image-bytes")}},
					},
				},
			}},
		},
	}, nil
}

func (m *mockAIClient) GetFile(ctx context.Context, name string) (*genai.File, error) {
	return &genai.File{Name: name, State: genai.FileStateActive}, nil
}

// --- Storage Reader Mock ---

type mockReader struct {
	data []byte
	err  error
}

func (m *mockReader) Open(ctx context.Context, uri string) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	d := m.data
	if d == nil {
		d = []byte("fake-storage-data")
	}
	return io.NopCloser(bytes.NewReader(d)), nil
}

func (m *mockReader) List(ctx context.Context, uri string, fn func(string) error) error {
	return nil
}

// --- HTTP Client Mock ---

type mockHTTPClient struct {
	data []byte
	err  error
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(m.data)),
	}, nil
}

func (m *mockHTTPClient) DoRequest(req *http.Request) ([]byte, error) {
	return m.data, m.err
}

func (m *mockHTTPClient) FetchBytes(ctx context.Context, url string) ([]byte, error) {
	if ok, err := m.IsSafeURL(url); !ok {
		return nil, fmt.Errorf("SSRF detection: %w", err)
	}
	return m.data, m.err
}

func (m *mockHTTPClient) FetchAndDecodeJSON(ctx context.Context, url string, v any) error {
	if m.err != nil {
		return m.err
	}
	return json.Unmarshal(m.data, v)
}

func (m *mockHTTPClient) PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error) {
	return m.data, m.err
}

func (m *mockHTTPClient) PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error) {
	return m.data, m.err
}

func (m *mockHTTPClient) IsSafeURL(urlStr string) (bool, error) {
	if strings.Contains(urlStr, "127.0.0.1") || strings.Contains(urlStr, "localhost") {
		return false, fmt.Errorf("restricted network access")
	}
	if urlStr == "" {
		return false, fmt.Errorf("empty URL")
	}
	return true, nil
}

func (m *mockHTTPClient) IsSecureServiceURL(serviceURL string) bool {
	return strings.Contains(serviceURL, "localhost") || strings.HasPrefix(serviceURL, "https://")
}

// --- Cache Mock ---

type mockCache struct {
	data map[string]any
}

func (m *mockCache) Get(key string) (any, bool) {
	if m.data == nil {
		return nil, false
	}
	val, ok := m.data[key]
	return val, ok
}

func (m *mockCache) Set(key string, value any, d time.Duration) {
	if m.data == nil {
		m.data = make(map[string]any)
	}
	m.data[key] = value
}
