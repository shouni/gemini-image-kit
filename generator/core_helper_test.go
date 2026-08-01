package generator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
)

// PrepareImageAttachment のテスト（キャッシュと変換）
func TestGeminiImageCore_PrepareImageAttachment(t *testing.T) {
	ctx := context.Background()
	cache := &mockCache{data: make(map[string]any)}
	// mocks_test.go の mockHTTPClient や mockReader を使用
	core := &GeminiImageCore{
		cache:      cache,
		httpClient: &mockHTTPClient{data: []byte("fake-image")},
		reader:     &mockReader{},
	}

	t.Run("キャッシュヒット時はURI参照を返す", func(t *testing.T) {
		rawURL := "https://example.com/img.png"
		fileURI := "https://generativelanguage.googleapis.com/v1beta/files/test-id"
		cache.Set(cacheKeyFileAPI+rawURL, cachedFile{URI: fileURI, Name: "files/test-id"}, time.Hour)

		attachment, err := core.PrepareImageAttachment(ctx, rawURL)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if attachment.URI != fileURI {
			t.Errorf("got %s, want %s", attachment.URI, fileURI)
		}
		// URI 参照のときはバイト列を持たない（同じ画像を二重に送らない）。
		if len(attachment.Data) != 0 {
			t.Error("expected no inline data for a cached URI reference")
		}
	})

	t.Run("画像として扱えないデータはエラーを返す", func(t *testing.T) {
		cache.Clear()
		attachment, err := core.PrepareImageAttachment(ctx, "https://example.com/not-image.png")
		if err == nil {
			t.Fatal("expected error for invalid image data")
		}
		if !attachment.IsEmpty() {
			t.Error("expected an empty attachment for invalid image data")
		}
	})

	t.Run("取得失敗はエラーを返す", func(t *testing.T) {
		cache.Clear()
		fetchErr := errors.New("network down")
		core := &GeminiImageCore{
			cache:      cache,
			httpClient: &mockHTTPClient{err: fetchErr},
			reader:     &mockReader{},
		}

		attachment, err := core.PrepareImageAttachment(ctx, "https://example.com/image.png")
		if err == nil {
			t.Fatal("expected fetch error")
		}
		if !attachment.IsEmpty() {
			t.Error("expected an empty attachment for fetch error")
		}
	})
}

// parseToResponse のテスト
func TestGeminiImageCore_parseToResponse(t *testing.T) {
	core := &GeminiImageCore{}
	seed := int64(999)

	t.Run("正常系: 画像が返る", func(t *testing.T) {
		resp := &gemini.Response{
			Attachments: []gemini.Attachment{{MIMEType: "image/png", Data: []byte("png-data")}},
		}

		out, err := core.parseToResponse(resp, seed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.MimeType != "image/png" || out.UsedSeed != seed {
			t.Errorf("parsed data mismatch: %+v", out)
		}
	})

	// NOTE: FinishReason の検証（安全ブロック等）は go-gemini-client 側が行うため、
	// parseToResponse では画像データの有無のみを検証します。
	t.Run("異常系: 添付が空（ブロック等で画像が返らなかった場合）", func(t *testing.T) {
		resp := &gemini.Response{}
		_, err := core.parseToResponse(resp, seed)
		if err == nil {
			t.Error("expected error when no image data is present")
		}
	})

	t.Run("異常系: 画像データなし（テキストのみ）", func(t *testing.T) {
		resp := &gemini.Response{Text: "just text"}
		_, err := core.parseToResponse(resp, seed)
		if err == nil {
			t.Error("expected error for text-only response")
		}
	})

	t.Run("異常系: 空のレスポンス", func(t *testing.T) {
		_, err := core.parseToResponse(nil, seed)
		if err == nil {
			t.Error("expected error for nil response")
		}
	})
}

// TestUploadDisplayName は、署名付き URL のクエリ文字列が表示名に混ざらないことを
// 確認します。filepath.Base をそのまま掛けると "ref.png?X-Goog-Signature=..." になります。
func TestUploadDisplayName(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{name: "GCS URI", uri: "gs://bucket/dir/ref.png", want: "ref.png"},
		{name: "署名付き URL", uri: "https://example.com/a/ref.png?X-Goog-Signature=abc&x=1", want: "ref.png"},
		{name: "フラグメント付き", uri: "https://example.com/ref.jpg#frag", want: "ref.jpg"},
		{name: "スキーム無し", uri: "dir/ref.webp", want: "ref.webp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uploadDisplayName(tt.uri); got != tt.want {
				t.Errorf("uploadDisplayName(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}
