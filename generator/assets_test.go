package generator

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 注意: mockAIClient, mockReader, mockHTTPClient, mockCache は
// mocks_test.go で定義されているため、ここでは定義不要です。
func TestGeminiImageCore_UploadFile(t *testing.T) {
	ctx := context.Background()
	// mocks_test.go のモックを利用
	cache := &mockCache{data: make(map[string]any)}
	ai := &mockAIClient{}
	pngHeader := []byte("\x89PNG\r\n\x1a\n")
	httpMock := &mockHTTPClient{data: append(pngHeader, []byte("fake-image-binary")...)}
	reader := &mockReader{}

	core, err := NewGeminiImageCore(GeminiImageCoreConfig{
		AIClient: ai, Reader: reader, HTTPClient: httpMock,
		Cache: cache, CacheTTL: time.Hour, Compress: false,
	})
	require.NoError(t, err, "failed to create core")

	t.Run("キャッシュがない場合はアップロードが実行される", func(t *testing.T) {
		ai.uploadCalled = false
		// 内部フィールドを直接触らず Clear メソッドを使用
		cache.Clear()
		fileURL := "https://example.com/test.png"

		uri, err := core.ensureUploaded(ctx, fileURL)

		require.NoError(t, err)
		assert.True(t, ai.uploadCalled, "expected AI client UploadFile to be called")

		// mocks_test.go の定数を使用
		assert.Equal(t, MockFileUploadURI, uri)

		// キャッシュに保存されているか確認
		cached, ok := cache.Get(cacheKeyFileAPI + fileURL)
		assert.True(t, ok, "should be cached")
		entry, ok := cached.(string)
		require.True(t, ok, "cache entry should be the uploaded URI string")
		assert.Equal(t, uri, entry)
	})

	t.Run("キャッシュがある場合はアップロードをスキップする", func(t *testing.T) {
		ai.uploadCalled = false
		fileURL := "https://example.com/cached.png"
		expectedURI := "https://generativelanguage.googleapis.com/v1beta/files/already-uploaded"
		cache.Set(cacheKeyFileAPI+fileURL, expectedURI, time.Hour)

		uri, err := core.ensureUploaded(ctx, fileURL)

		require.NoError(t, err)
		assert.False(t, ai.uploadCalled, "AI client UploadFile should NOT be called when cached")
		assert.Equal(t, expectedURI, uri)
	})
}

func TestGeminiImageCore_UploadFile_MIMEHandling(t *testing.T) {
	ctx := context.Background()
	reader := &mockReader{}

	t.Run("拡張子ではなく実データのMIMEでアップロードする", func(t *testing.T) {
		cache := &mockCache{data: make(map[string]any)}
		ai := &mockAIClient{}
		pngHeader := []byte("\x89PNG\r\n\x1a\n")
		httpMock := &mockHTTPClient{data: append(pngHeader, []byte("fake-image-binary")...)}
		core, err := NewGeminiImageCore(GeminiImageCoreConfig{
			AIClient: ai, Reader: reader, HTTPClient: httpMock,
			Cache: cache, CacheTTL: time.Hour, Compress: false,
		})
		require.NoError(t, err)

		_, err = core.ensureUploaded(ctx, "https://example.com/no-extension")

		require.NoError(t, err)
		assert.Equal(t, "image/png", ai.lastUploadMIMEType)
	})

	t.Run("圧縮後はJPEGのMIMEでアップロードする", func(t *testing.T) {
		cache := &mockCache{data: make(map[string]any)}
		ai := &mockAIClient{}
		httpMock := &mockHTTPClient{data: createPNGData(t)}
		core, err := NewGeminiImageCore(GeminiImageCoreConfig{
			AIClient: ai, Reader: reader, HTTPClient: httpMock,
			Cache: cache, CacheTTL: time.Hour, Compress: true,
		})
		require.NoError(t, err)

		_, err = core.ensureUploaded(ctx, "https://example.com/input.png")

		require.NoError(t, err)
		assert.Equal(t, "image/jpeg", ai.lastUploadMIMEType)
		_, format, err := image.Decode(bytes.NewReader(ai.lastUploadData))
		require.NoError(t, err)
		assert.Equal(t, "jpeg", format)
	})
}

func createPNGData(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for x := 0; x < 2; x++ {
		for y := 0; y < 2; y++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}

	buf := new(bytes.Buffer)
	require.NoError(t, png.Encode(buf, img))
	return buf.Bytes()
}
