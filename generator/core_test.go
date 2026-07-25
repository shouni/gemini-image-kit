package generator

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shouni/gemini-image-kit/ports"
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

		uri, err := core.EnsureUploaded(ctx, fileURL)

		require.NoError(t, err)
		assert.True(t, ai.uploadCalled, "expected AI client UploadFile to be called")

		// mocks_test.go の定数を使用
		assert.Equal(t, MockFileUploadURI, uri)

		// キャッシュに保存されているか確認
		cached, ok := cache.Get(cacheKeyFileAPI + fileURL)
		assert.True(t, ok, "should be cached")
		entry, ok := cached.(cachedFile)
		require.True(t, ok, "cache entry should be a cachedFile")
		assert.Equal(t, uri, entry.URI)
		assert.Equal(t, MockFileUploadName, entry.Name, "削除に必要な Name も同じエントリに入っていること")
	})

	t.Run("キャッシュがある場合はアップロードをスキップする", func(t *testing.T) {
		ai.uploadCalled = false
		fileURL := "https://example.com/cached.png"
		expectedURI := "https://generativelanguage.googleapis.com/v1beta/files/already-uploaded"
		cache.Set(cacheKeyFileAPI+fileURL, cachedFile{URI: expectedURI, Name: "files/already-uploaded"}, time.Hour)

		uri, err := core.EnsureUploaded(ctx, fileURL)

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

		_, err = core.EnsureUploaded(ctx, "https://example.com/no-extension")

		require.NoError(t, err)
		assert.Equal(t, "image/png", ai.lastUploadMIMEType)
	})

	t.Run("圧縮後はJPEGのMIMEでアップロードする", func(t *testing.T) {
		cache := &mockCache{data: make(map[string]any)}
		ai := &mockAIClient{}
		httpMock := &mockHTTPClient{data: createPNGData(t)}
		core, err := NewGeminiImageCore(ai, reader, httpMock, cache, time.Hour, true)
		require.NoError(t, err)

		_, err = core.EnsureUploaded(ctx, "https://example.com/input.png")

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
		cache.Set(cacheKeyFileAPI+fileURL, cachedFile{URI: "https://example.com/files/x", Name: apiName}, time.Hour)

		err := core.DeleteFile(ctx, fileURL)

		require.NoError(t, err)
		assert.Equal(t, apiName, ai.lastFileName)
	})

	t.Run("削除に成功したらキャッシュも無効化される", func(t *testing.T) {
		fileURL := "https://example.com/invalidate.png"
		cache.Set(cacheKeyFileAPI+fileURL, cachedFile{URI: "https://example.com/files/dead", Name: "files/dead-id"}, time.Hour)

		err := core.DeleteFile(ctx, fileURL)

		require.NoError(t, err)
		_, ok := cache.Get(cacheKeyFileAPI + fileURL)
		assert.False(t, ok, "URI cache entry should be invalidated after deletion")
		_, ok = cache.Get(cacheKeyFileAPI + fileURL)
		assert.False(t, ok, "name cache entry should be invalidated after deletion")
	})

	t.Run("キャッシュがない場合はErrFileNotInCacheを返す", func(t *testing.T) {
		rawID := "files/raw-id"
		// キャッシュをクリアした状態で実行
		cache.Clear()
		err := core.DeleteFile(ctx, rawID)

		// assert.Error ではなく require.Error を使用し、nil パニックを防ぐ
		require.Error(t, err, "expected error when cache is missing")
		assert.ErrorIs(t, err, ErrFileNotInCache)
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

// TestNewGeminiImageCore_RequiredDependencies は、必須依存の nil チェックを検証します。
//
// cache は「あれば速くなる」任意の依存に見えますが、DeleteFile が File API 上の
// ファイル名をここから引くため、nil だと削除が一切できずサーバー側にファイルが
// 残り続けます。したがって必須です。
func TestNewGeminiImageCore_RequiredDependencies(t *testing.T) {
	ai := &mockAIClient{}
	reader := &mockReader{}
	httpClient := &mockHTTPClient{}
	cache := &mockCache{}

	tests := []struct {
		name    string
		ai      gemini.GenerativeModel
		reader  ports.ContentReader
		http    ports.Downloader
		cache   ports.ImageCacher
		wantErr error
	}{
		{"aiClient なし", nil, reader, httpClient, cache, ErrAIClientRequired},
		{"reader なし", ai, nil, httpClient, cache, ErrReaderRequired},
		{"httpClient なし", ai, reader, nil, cache, ErrHTTPClientRequired},
		{"cache なし", ai, reader, httpClient, nil, ErrCacheRequired},
		{"すべて揃っている", ai, reader, httpClient, cache, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, err := NewGeminiImageCore(tt.ai, tt.reader, tt.http, tt.cache, time.Hour, false)

			if tt.wantErr == nil {
				require.NoError(t, err)
				assert.NotNil(t, core)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, core)
		})
	}
}
