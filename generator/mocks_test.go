package generator

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"github.com/stretchr/testify/require"

	"github.com/shouni/gemini-image-kit/ports"
)

const (
	MockFileUploadURI  = "https://generativelanguage.googleapis.com/v1beta/files/mock-id"
	MockFileUploadName = "files/mock-id"
)

// --- 生成クライアント ---

// fakeClient は gemini.Generator（+ BackendInspector）のテストダブルです。
// 送信されたプロンプト・添付・オプションを記録します。参照画像の解決が並行に
// 走るため、記録フィールドは mu で保護します（-race 対策）。
type fakeClient struct {
	vertexAI bool
	// generate を設定すると生成の挙動を差し替えられます（遅延・失敗の注入用）。
	generate func(ctx context.Context, model, prompt string) (*gemini.Response, error)

	mu              sync.Mutex
	generateCalled  bool
	lastPrompt      string
	lastAttachments []gemini.Attachment
	lastOptions     gemini.GenerateOptions
}

func (c *fakeClient) IsVertexAI() bool { return c.vertexAI }

func (c *fakeClient) GenerateWithAttachments(ctx context.Context, model string, prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions) (*gemini.Response, error) {
	c.mu.Lock()
	c.generateCalled = true
	c.lastPrompt = prompt
	c.lastAttachments = attachments
	c.lastOptions = opts
	c.mu.Unlock()

	if c.generate != nil {
		return c.generate(ctx, model, prompt)
	}
	return &gemini.Response{
		Attachments: []gemini.Attachment{{MIMEType: "image/png", Data: []byte("fake-image-bytes")}},
	}, nil
}

func (c *fakeClient) attachments() []gemini.Attachment {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastAttachments
}

func (c *fakeClient) options() gemini.GenerateOptions {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastOptions
}

// --- File API ---

// fakeUploader は gemini.FileManager のテストダブルです。
type fakeUploader struct {
	mu                 sync.Mutex
	uploadCalled       bool
	lastUploadMIMEType string
	lastUploadData     []byte
}

func (m *fakeUploader) UploadFile(_ context.Context, r io.Reader, mimeType, _ string) (gemini.UploadedFile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return gemini.UploadedFile{}, err
	}

	m.mu.Lock()
	m.uploadCalled = true
	m.lastUploadMIMEType = mimeType
	m.lastUploadData = data
	m.mu.Unlock()
	return gemini.UploadedFile{URI: MockFileUploadURI, Name: MockFileUploadName}, nil
}

func (m *fakeUploader) DeleteFile(_ context.Context, _ string) error { return nil }

func (m *fakeUploader) called() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.uploadCalled
}

func (m *fakeUploader) mimeType() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastUploadMIMEType
}

// countingUploader は、アップロード回数を数えるための FileManager です。
// 重複アップロードの検証には呼び出し回数そのものが必要なので、bool フラグを持つ
// fakeUploader とは別に用意しています。
type countingUploader struct {
	fakeUploader
	uploads     atomic.Int64
	uploadDelay time.Duration
	uploadErr   error
}

func (m *countingUploader) UploadFile(_ context.Context, r io.Reader, mimeType, _ string) (gemini.UploadedFile, error) {
	if _, err := io.ReadAll(r); err != nil {
		return gemini.UploadedFile{}, err
	}
	if m.uploadDelay > 0 {
		time.Sleep(m.uploadDelay)
	}
	if m.uploadErr != nil {
		return gemini.UploadedFile{}, m.uploadErr
	}
	m.uploads.Add(1)

	m.mu.Lock()
	m.uploadCalled = true
	m.lastUploadMIMEType = mimeType
	m.mu.Unlock()
	return gemini.UploadedFile{URI: MockFileUploadURI, Name: MockFileUploadName}, nil
}

// --- 取得系 ---

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

type mockHTTPClient struct {
	data []byte
	err  error
}

func (m *mockHTTPClient) GetStream(_ context.Context, _ string) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(bytes.NewReader(m.data)), nil
}

// --- キャッシュ ---

// mockCache は ports.ImageCacher のテストダブルです。参照画像の解決は並行に走るため、
// 本物の実装と同じく同時アクセス安全である必要があります（素の map のままだと
// -race で落ちます）。
type mockCache struct {
	mu   sync.RWMutex
	data map[string]any
}

func (m *mockCache) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]any)
}

func (m *mockCache) Get(key string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.data == nil {
		return nil, false
	}
	val, ok := m.data[key]
	return val, ok
}

func (m *mockCache) Set(key string, value any, _ time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = make(map[string]any)
	}
	m.data[key] = value
}

// --- Resolver ---

// stubResolver は ports.ReferenceResolver のテストダブルです。参照解決だけを
// 差し替えられるので、並行実行や順序の検証をネットワーク無しで行えます。
type stubResolver struct {
	// resolve は参照の解決を差し替えます。nil なら URL をそのままデータに
	// 載せた添付を返します。
	resolve func(ctx context.Context, rawURL string) (gemini.Attachment, error)
}

func (s *stubResolver) Resolve(ctx context.Context, uri ports.ImageURI) (gemini.Attachment, error) {
	if s.resolve != nil {
		return s.resolve(ctx, uri.ReferenceURL)
	}
	return gemini.Attachment{MIMEType: "image/png", Data: []byte(uri.ReferenceURL)}, nil
}

// newStubGenerator は、参照解決をスタブに差し替えた Generator と、送信内容を
// 記録するクライアントを返します。
func newStubGenerator(t *testing.T, resolver ports.ReferenceResolver, opts ...Option) (*Generator, *fakeClient) {
	t.Helper()
	client := &fakeClient{}
	g, err := New(client, resolver, opts...)
	require.NoError(t, err)
	return g, client
}

// --- テストデータ ---

func createPNGData(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for x := range 2 {
		for y := range 2 {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}

	buf := new(bytes.Buffer)
	require.NoError(t, png.Encode(buf, img))
	return buf.Bytes()
}
