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

	core, err := NewGeminiImageCore(ai, reader, httpMock, cache, time.Hour, false)
	require.NoError(t, err, "failed to create core")

	t.Run("キャッシュがない場合はアップロードが実行される", func(t *testing.T) {
		ai.uploadCalled = false
		// 内部フィールドを直接触らず Clear メソッドを使用
		cache.Clear()
		fileURL := "https://example.com/test.png"

		uri, err := core.UploadFile(ctx, fileURL)

		require.NoError(t, err)
		assert.True(t, ai.uploadCalled, "expected AI client UploadFile to be called")

		// mocks_test.go の定数を使用
		assert.Equal(t, MockFileUploadURI, uri)

		// キャッシュに保存されているか確認
		cachedURI, ok := cache.Get(cacheKeyFileAPIURI + fileURL)
		assert.True(t, ok, "should be cached")
		assert.Equal(t, uri, cachedURI)
	})

	t.Run("キャッシュがある場合はアップロードをスキップする", func(t *testing.T) {
		ai.uploadCalled = false
		fileURL := "https://example.com/cached.png"
		expectedURI := "https://generativelanguage.googleapis.com/v1beta/files/already-uploaded"
		cache.Set(cacheKeyFileAPIURI+fileURL, expectedURI, time.Hour)

		uri, err := core.UploadFile(ctx, fileURL)

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
		core, err := NewGeminiImageCore(ai, reader, httpMock, cache, time.Hour, false)
		require.NoError(t, err)

		_, err = core.UploadFile(ctx, "https://example.com/no-extension")

		require.NoError(t, err)
		assert.Equal(t, "image/png", ai.lastUploadMIMEType)
	})

	t.Run("圧縮後はJPEGのMIMEでアップロードする", func(t *testing.T) {
		cache := &mockCache{data: make(map[string]any)}
		ai := &mockAIClient{}
		httpMock := &mockHTTPClient{data: createPNGData(t)}
		core, err := NewGeminiImageCore(ai, reader, httpMock, cache, time.Hour, true)
		require.NoError(t, err)

		_, err = core.UploadFile(ctx, "https://example.com/input.png")

		require.NoError(t, err)
		assert.Equal(t, "image/jpeg", ai.lastUploadMIMEType)
		_, format, err := image.Decode(bytes.NewReader(ai.lastUploadData))
		require.NoError(t, err)
		assert.Equal(t, "jpeg", format)
	})
}

func TestGeminiImageCore_DeleteFile(t *testing.T) {
	ctx := context.Background()
	cache := &mockCache{data: make(map[string]any)}
	ai := &mockAIClient{}
	reader := &mockReader{}

	// 修正: 圧縮設定を false に統一
	core, _ := NewGeminiImageCore(ai, reader, &mockHTTPClient{}, cache, time.Hour, false)

	t.Run("キャッシュから名前を引いて削除に成功する", func(t *testing.T) {
		fileURL := "https://example.com/image.png"
		apiName := "files/specific-id"
		// 削除にはこのキャッシュが必須
		cache.Set(cacheKeyFileAPIName+fileURL, apiName, time.Hour)

		err := core.DeleteFile(ctx, fileURL)

		require.NoError(t, err)
		assert.Equal(t, apiName, ai.lastFileName)
	})

	t.Run("キャッシュがない場合はエラーを返す", func(t *testing.T) {
		rawID := "files/raw-id"
		// キャッシュをクリアした状態で実行
		cache.Clear()
		err := core.DeleteFile(ctx, rawID)

		//assert.Error ではなく require.Error を使用し、nil パニックを防ぐ
		require.Error(t, err, "expected error when cache is missing")

		// エラーメッセージの検証
		expectedErrMsg := "cannot determine file name for deletion"
		assert.Contains(t, err.Error(), expectedErrMsg)
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
