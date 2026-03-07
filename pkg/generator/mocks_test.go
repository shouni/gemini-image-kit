package generator

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
)

// --- AI Client Mock ---

const (
	MockFileUploadURI  = "https://generativelanguage.googleapis.com/v1beta/files/mock-id"
	MockFileUploadName = "files/mock-id"
)

type mockAIClient struct {
	uploadCalled bool
	deleteCalled bool
	lastFileName string
	backend      genai.Backend
}

// IsVertexAI を実装
func (m *mockAIClient) IsVertexAI() bool {
	return m.backend == genai.BackendVertexAI
}

func (m *mockAIClient) UploadFile(ctx context.Context, r io.Reader, mimeType, displayName string) (string, string, error) {
	_, err := io.ReadAll(r)
	if err != nil {
		return "", "", err
	}

	m.uploadCalled = true
	return MockFileUploadURI, MockFileUploadName, nil
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

// (既存の Do, DoRequest, FetchBytes, FetchAndDecodeJSON, PostJSONAndFetchBytes, PostRawBodyAndFetchBytes, IsSafeURL, IsSecureServiceURL はそのまま)

// FetchStream の実装
func (m *mockHTTPClient) FetchStream(ctx context.Context, url string, fn func(io.Reader) error) error {
	if m.err != nil {
		return m.err
	}
	// テスト用に m.data を流し込む
	return fn(bytes.NewReader(m.data))
}

// GetStream の実装
func (m *mockHTTPClient) GetStream(ctx context.Context, url string) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	// ストリームを返すため、io.NopCloser でラップする
	return io.NopCloser(bytes.NewReader(m.data)), nil
}

// --- Cache Mock ---

type mockCache struct {
	data map[string]any
}

func (m *mockCache) Clear() {
	m.data = make(map[string]any)
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
