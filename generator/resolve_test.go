package generator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shouni/gemini-image-kit/ports"
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

func newPolicyCore(t *testing.T, cfg GeminiImageCoreConfig) *GeminiImageCore {
	t.Helper()
	png := createPNGData(t)
	if cfg.Reader == nil {
		cfg.Reader = &mockReader{data: png}
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &mockHTTPClient{data: png}
	}
	if cfg.Cache == nil {
		cfg.Cache = &mockCache{}
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = time.Hour
	}
	core, err := NewGeminiImageCore(cfg)
	if err != nil {
		t.Fatalf("NewGeminiImageCore() error = %v", err)
	}
	return core
}

// TestResolveReferenceUploadsOnGeminiAPI は、Gemini API バックエンドでは参照画像を
// File API へ上げて URI 参照になることを確認します。毎回インラインで送ると、
// 同じ参照画像を使い回すワークロードで同じバイト列を何度も送ることになります。
func TestResolveReferenceUploadsOnGeminiAPI(t *testing.T) {
	ai := &mockAIClient{vertexAI: false}
	core := newPolicyCore(t, GeminiImageCoreConfig{AIClient: ai})

	attachment, err := core.ResolveReference(context.Background(), ports.ImageURI{ReferenceURL: "https://example.com/char.png"})
	if err != nil {
		t.Fatalf("ResolveReference() error = %v", err)
	}
	if !ai.uploadCalled {
		t.Error("File API へアップロードされていません")
	}
	if attachment.URI != MockFileUploadURI {
		t.Errorf("URI = %q, want %q", attachment.URI, MockFileUploadURI)
	}
	if len(attachment.Data) != 0 {
		t.Error("URI 参照なのにバイト列が載っています")
	}
}

// TestResolveReferenceReusesUpload は、2回目以降がキャッシュから返り、
// アップロードが繰り返されないことを確認します。
func TestResolveReferenceReusesUpload(t *testing.T) {
	ai := &countingAIClient{}
	core := newPolicyCore(t, GeminiImageCoreConfig{AIClient: ai})
	uri := ports.ImageURI{ReferenceURL: "https://example.com/char.png"}

	for i := range 3 {
		if _, err := core.ResolveReference(context.Background(), uri); err != nil {
			t.Fatalf("ResolveReference() #%d error = %v", i, err)
		}
	}
	if got := ai.uploads.Load(); got != 1 {
		t.Errorf("uploads = %d, want 1", got)
	}
}

// TestEnsureUploadedDeduplicatesConcurrentUploads は、同一ソースへの同時呼び出しが
// 1回のアップロードにまとまることを確認します。キャッシュは完了後にしか書かれないため、
// singleflight が無いと並行呼び出しの数だけ File API 上に重複ファイルができます。
func TestEnsureUploadedDeduplicatesConcurrentUploads(t *testing.T) {
	ai := &countingAIClient{uploadDelay: 20 * time.Millisecond}
	core := newPolicyCore(t, GeminiImageCoreConfig{AIClient: ai})

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := core.EnsureUploaded(context.Background(), "https://example.com/same.png"); err != nil {
				t.Errorf("EnsureUploaded() error = %v", err)
			}
		}()
	}
	wg.Wait()

	if got := ai.uploads.Load(); got != 1 {
		t.Errorf("uploads = %d, want 1 (concurrent calls must share one upload)", got)
	}
}

// TestResolveReferenceInlinesOnVertexNonGCS は、Vertex AI で gs:// 以外を参照した場合に
// インライン送信になることを確認します。Vertex AI に File API は無いためです。
func TestResolveReferenceInlinesOnVertexNonGCS(t *testing.T) {
	ai := &mockAIClient{vertexAI: true}
	core := newPolicyCore(t, GeminiImageCoreConfig{AIClient: ai})

	attachment, err := core.ResolveReference(context.Background(), ports.ImageURI{ReferenceURL: "https://example.com/char.png"})
	if err != nil {
		t.Fatalf("ResolveReference() error = %v", err)
	}
	if ai.uploadCalled {
		t.Error("Vertex AI では File API を使ってはいけません")
	}
	if len(attachment.Data) == 0 {
		t.Error("インラインのバイト列が載っていません")
	}
}

// TestResolveReferencePassesThroughGCSOnVertex は、Vertex AI + gs:// が転送なしの
// 直接参照になることを確認します（最も安い経路なので FileAPIURI より優先します）。
func TestResolveReferencePassesThroughGCSOnVertex(t *testing.T) {
	ai := &mockAIClient{vertexAI: true}
	core := newPolicyCore(t, GeminiImageCoreConfig{AIClient: ai})

	attachment, err := core.ResolveReference(context.Background(), ports.ImageURI{
		ReferenceURL: "gs://bucket/char.png",
		FileAPIURI:   "https://generativelanguage.googleapis.com/v1beta/files/ignored",
	})
	if err != nil {
		t.Fatalf("ResolveReference() error = %v", err)
	}
	if attachment.URI != "gs://bucket/char.png" {
		t.Errorf("URI = %q, want the gs:// reference", attachment.URI)
	}
	if ai.uploadCalled {
		t.Error("gs:// の直接参照でアップロードが走っています")
	}
}

// TestResolveReferenceInlineReferencesOption は、InlineReferences で従来どおりの
// インライン送信に固定できることを確認します（使い捨ての参照画像向け）。
func TestResolveReferenceInlineReferencesOption(t *testing.T) {
	ai := &mockAIClient{vertexAI: false}
	core := newPolicyCore(t, GeminiImageCoreConfig{AIClient: ai, InlineReferences: true})

	attachment, err := core.ResolveReference(context.Background(), ports.ImageURI{ReferenceURL: "https://example.com/char.png"})
	if err != nil {
		t.Fatalf("ResolveReference() error = %v", err)
	}
	if ai.uploadCalled {
		t.Error("InlineReferences 指定時にアップロードが走っています")
	}
	if len(attachment.Data) == 0 {
		t.Error("インラインのバイト列が載っていません")
	}
}

// TestResolveReferenceFallsBackToInlineOnUploadFailure は、アップロードが失敗しても
// インライン送信で生成を続行できることを確認します。アップロードは送信量を減らす
// 最適化なので、その失敗で生成そのものを落とす理由はありません。
func TestResolveReferenceFallsBackToInlineOnUploadFailure(t *testing.T) {
	ai := &countingAIClient{uploadErr: errors.New("quota exceeded")}
	core := newPolicyCore(t, GeminiImageCoreConfig{AIClient: ai})

	attachment, err := core.ResolveReference(context.Background(), ports.ImageURI{ReferenceURL: "https://example.com/char.png"})
	if err != nil {
		t.Fatalf("ResolveReference() error = %v", err)
	}
	if len(attachment.Data) == 0 {
		t.Error("アップロード失敗時にインラインへフォールバックしていません")
	}
}

// TestResolveReferenceSkipsEmpty は、参照先を持たない要素が空の添付になることを
// 確認します（テキストのみの生成で使われるため、エラーにはしません）。
func TestResolveReferenceSkipsEmpty(t *testing.T) {
	core := newPolicyCore(t, GeminiImageCoreConfig{AIClient: &mockAIClient{}})

	attachment, err := core.ResolveReference(context.Background(), ports.ImageURI{})
	if err != nil {
		t.Fatalf("ResolveReference() error = %v", err)
	}
	if !attachment.IsEmpty() {
		t.Errorf("attachment = %+v, want an empty one", attachment)
	}
}
