package generator

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
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
	lastPrompt         string
	lastAttachments    []gemini.Attachment
	lastFileName       string
	lastUploadMIMEType string
	lastUploadData     []byte
	vertexAI           bool
}

// IsVertexAI を実装
func (m *mockAIClient) IsVertexAI() bool {
	return m.vertexAI
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

func (m *mockAIClient) GenerateWithAttachments(_ context.Context, _ string, prompt string, attachments []gemini.Attachment, _ gemini.GenerateOptions) (*gemini.Response, error) {
	m.generateCalled = true
	m.lastPrompt = prompt
	m.lastAttachments = attachments
	return &gemini.Response{
		Attachments: []gemini.Attachment{{MIMEType: "image/png", Data: []byte("fake-image-bytes")}},
	}, nil
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
