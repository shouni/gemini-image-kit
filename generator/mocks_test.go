package generator

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
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

// --- ImageExecutor Mock ---

// stubExecutor は ports.ImageExecutor のテストダブルです。参照画像の解決だけを
// 差し替えられるので、並行実行や順序の検証をネットワーク無しで行えます。
type stubExecutor struct {
	vertexAI bool

	// prepare は PrepareImageAttachment の挙動を差し替えます。nil なら URL を
	// そのままデータに載せた添付を返します。
	prepare func(ctx context.Context, rawURL string) (gemini.Attachment, error)

	lastPrompt      string
	lastAttachments []gemini.Attachment
	lastOptions     gemini.GenerateOptions
}

func (s *stubExecutor) IsVertexAI() bool { return s.vertexAI }

func (s *stubExecutor) ExecuteRequest(_ context.Context, _ string, prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions) (*ports.ImageResponse, error) {
	s.lastPrompt = prompt
	s.lastAttachments = attachments
	s.lastOptions = opts
	return &ports.ImageResponse{
		Data:     []byte("stub-image"),
		MimeType: "image/png",
		UsedSeed: ports.DereferenceSeed(opts.Seed),
	}, nil
}

func (s *stubExecutor) PrepareImageAttachment(ctx context.Context, rawURL string) (gemini.Attachment, error) {
	if s.prepare != nil {
		return s.prepare(ctx, rawURL)
	}
	return gemini.Attachment{MIMEType: "image/png", Data: []byte(rawURL)}, nil
}
