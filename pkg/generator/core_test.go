package generator

import (
	"context"
	"strings"
	"testing"
	"time"
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
	if err != nil {
		t.Fatalf("failed to create core: %v", err)
	}

	t.Run("キャッシュがない場合はアップロードが実行される", func(t *testing.T) {
		ai.uploadCalled = false
		fileURL := "https://example.com/test.png"

		uri, err := core.UploadFile(ctx, fileURL)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ai.uploadCalled {
			t.Error("expected AI client UploadFile to be called")
		}
		if uri != "https://gemini.api/files/new-file-id" {
			t.Errorf("got uri %s, want https://gemini.api/files/new-file-id", uri)
		}

		// キャッシュに保存されているか確認
		cachedURI, _ := cache.Get(cacheKeyFileAPIURI + fileURL)
		if cachedURI != uri {
			t.Errorf("cache mismatch: got %v, want %v", cachedURI, uri)
		}
	})

	t.Run("キャッシュがある場合はアップロードをスキップする", func(t *testing.T) {
		ai.uploadCalled = false
		fileURL := "https://example.com/cached.png"
		expectedURI := "https://gemini.api/files/already-uploaded"
		cache.Set(cacheKeyFileAPIURI+fileURL, expectedURI, time.Hour)

		uri, err := core.UploadFile(ctx, fileURL)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ai.uploadCalled {
			t.Error("AI client UploadFile should NOT be called when cached")
		}
		if uri != expectedURI {
			t.Errorf("got uri %s, want %s", uri, expectedURI)
		}
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

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ai.lastFileName != apiName {
			t.Errorf("expected %s, got %s", apiName, ai.lastFileName)
		}
	})

	t.Run("キャッシュがない場合はエラーを返す（仕様変更の確認）", func(t *testing.T) {
		rawID := "files/raw-id"
		// キャッシュに何も入れずに実行
		err := core.DeleteFile(ctx, rawID)

		if err == nil {
			t.Error("expected error when cache is missing, but got nil")
		}

		expectedErrMsg := "cannot determine file name for deletion"
		if err != nil && !strings.Contains(err.Error(), expectedErrMsg) {
			t.Errorf("expected error message to contain %q, got %q", expectedErrMsg, err.Error())
		}
	})
}
