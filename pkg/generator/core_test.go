package generator

import (
	"context"
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
	httpMock := &mockHTTPClient{data: []byte("fake-image-binary")}
	reader := &mockReader{}

	core, err := NewGeminiImageCore(ai, reader, httpMock, cache, time.Hour)
	require.NoError(t, err, "failed to create core")

	// モック (mockAIClient.UploadFile) が返す期待値
	const mockURI = "https://generativelanguage.googleapis.com/v1beta/files/mock-id"

	t.Run("キャッシュがない場合はアップロードが実行される", func(t *testing.T) {
		ai.uploadCalled = false
		fileURL := "https://example.com/test.png"

		uri, err := core.UploadFile(ctx, fileURL)

		require.NoError(t, err)
		assert.True(t, ai.uploadCalled, "expected AI client UploadFile to be called")

		// 期待値を mocks_test.go の戻り値に合わせる
		assert.Equal(t, mockURI, uri)

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

func TestGeminiImageCore_DeleteFile(t *testing.T) {
	ctx := context.Background()
	cache := &mockCache{data: make(map[string]any)}
	ai := &mockAIClient{}
	reader := &mockReader{}

	core, _ := NewGeminiImageCore(ai, reader, &mockHTTPClient{}, cache, time.Hour)

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
		// キャッシュに何も入れずに実行
		err := core.DeleteFile(ctx, rawID)

		assert.Error(t, err, "expected error when cache is missing")

		// エラーメッセージの検証
		expectedErrMsg := "cannot determine file name for deletion"
		assert.Contains(t, err.Error(), expectedErrMsg)
	})
}
