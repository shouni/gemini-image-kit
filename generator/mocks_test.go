package generator

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"google.golang.org/genai"
)

// --- AI Client Mock ---

const (
	MockFileUploadURI  = "https://generativelanguage.googleapis.com/v1beta/files/mock-id"
	MockFileUploadName = "files/mock-id"
)

type mockAIClient struct {
	uploadCalled       bool
	deleteCalled       bool
	generateCalled     bool
	lastFileName       string
	lastUploadMIMEType string
	lastUploadData     []byte
	backend            genai.Backend
}

// IsVertexAI を実装
func (m *mockAIClient) IsVertexAI() bool {
	return m.backend == genai.BackendVertexAI
}

func (m *mockAIClient) UploadFile(_ context.Context, r io.Reader, mimeType, _ string) (gemini.UploadedFile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return gemini.UploadedFile{}, err
	}

	m.uploadCalled = true
	m.lastUploadMIMEType = mimeType
	m.lastUploadData = data
	return gemini.UploadedFile{URI: MockFileUploadURI, Name: MockFileUploadName}, nil
}

func (m *mockAIClient) DeleteFile(_ context.Context, name string) error {
	m.deleteCalled = true
	m.lastFileName = name
	return nil
}

func (m *mockAIClient) GenerateContent(_ context.Context, _ string, _ string) (*gemini.Response, error) {
	return nil, nil
}

func (m *mockAIClient) GenerateWithParts(_ context.Context, _ string, _ []*genai.Part, _ gemini.GenerateOptions) (*gemini.Response, error) {
	m.generateCalled = true
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

func (m *mockAIClient) GetFile(_ context.Context, name string) (*genai.File, error) {
	return &genai.File{Name: name, State: genai.FileStateActive}, nil
}

// --- Storage Reader Mock ---

type mockReader struct {
	data []byte
	err  error
}

func (m *mockReader) Open(_ context.Context, _ string) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	d := m.data
	if d == nil {
		d = []byte("fake-storage-data")
	}
	return io.NopCloser(bytes.NewReader(d)), nil
}

// --- HTTP Client Mock ---

type mockHTTPClient struct {
	data []byte
	err  error
}

// GetStream の実装
func (m *mockHTTPClient) GetStream(_ context.Context, _ string) (io.ReadCloser, error) {
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

func (m *mockCache) Set(key string, value any, _ time.Duration) {
	if m.data == nil {
		m.data = make(map[string]any)
	}
	m.data[key] = value
}

func (m *mockCache) Delete(key string) {
	delete(m.data, key)
}
