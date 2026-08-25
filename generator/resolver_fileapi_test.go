package generator

import (
	"bytes"
	"context"
	"errors"
	"image"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shouni/gemini-image-kit/ports"
)

func newTestFileAPIResolver(t *testing.T, cfg FileAPIResolverConfig) *FileAPIResolver {
	t.Helper()
	png := createPNGData(t)
	if cfg.Files == nil {
		cfg.Files = &fakeUploader{}
	}
	if cfg.Reader == nil {
		cfg.Reader = &mockReader{data: png}
	}
	if cfg.Downloader == nil {
		cfg.Downloader = &mockHTTPClient{data: png}
	}
	if cfg.Cache == nil {
		cfg.Cache = &mockCache{}
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = time.Hour
	}
	r, err := NewFileAPIResolver(cfg)
	require.NoError(t, err)
	return r
}

// TestFileAPIResolverRequiredDependencies は、必須依存の nil チェックを検証します。
//
// Cache が必須なのは、アップロード済み参照の使い回しがこの resolver の存在理由
// そのものであり、キャッシュ無しでは毎回上げ直すだけになるためです。
func TestFileAPIResolverRequiredDependencies(t *testing.T) {
	full := func() FileAPIResolverConfig {
		return FileAPIResolverConfig{
			Files: &fakeUploader{}, Reader: &mockReader{},
			Downloader: &mockHTTPClient{}, Cache: &mockCache{}, CacheTTL: time.Hour,
		}
	}

	tests := []struct {
		name    string
		mutate  func(*FileAPIResolverConfig)
		wantErr error
	}{
		{"Files なし", func(c *FileAPIResolverConfig) { c.Files = nil }, ErrFileManagerRequired},
		{"Reader なし", func(c *FileAPIResolverConfig) { c.Reader = nil }, ErrReaderRequired},
		{"Downloader なし", func(c *FileAPIResolverConfig) { c.Downloader = nil }, ErrHTTPClientRequired},
		{"Cache なし", func(c *FileAPIResolverConfig) { c.Cache = nil }, ErrCacheRequired},
		{"すべて揃っている", func(*FileAPIResolverConfig) {}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := full()
			tt.mutate(&cfg)
			r, err := NewFileAPIResolver(cfg)

			if tt.wantErr == nil {
				require.NoError(t, err)
				assert.NotNil(t, r)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, r)
		})
	}
}

// TestFileAPIResolverUploadsAndReferences は、参照画像が File API へ上がって
// URI 参照になることを確認します。毎回インラインで送ると、同じ参照画像を使い回す
// ワークロードで同じバイト列を何度も送ることになります。
func TestFileAPIResolverUploadsAndReferences(t *testing.T) {
	files := &fakeUploader{}
	r := newTestFileAPIResolver(t, FileAPIResolverConfig{Files: files})

	attachment, err := r.Resolve(context.Background(), ports.ImageURI{ReferenceURL: "https://example.com/char.png"})
	require.NoError(t, err)

	assert.True(t, files.called(), "File API へアップロードされていません")
	assert.Equal(t, MockFileUploadURI, attachment.URI)
	assert.Empty(t, attachment.Data, "URI 参照なのにバイト列が載っています")
}

// TestFileAPIResolverPrefersProvidedURI は、呼び出し側が既にアップロード済みの
// URI を持っている場合にアップロードを省くことを確認します。
func TestFileAPIResolverPrefersProvidedURI(t *testing.T) {
	files := &fakeUploader{}
	r := newTestFileAPIResolver(t, FileAPIResolverConfig{Files: files})

	const uploaded = "https://generativelanguage.googleapis.com/v1beta/files/abc-123"
	attachment, err := r.Resolve(context.Background(), ports.ImageURI{
		ReferenceURL: "https://example.com/ignore.jpg",
		FileAPIURI:   uploaded,
	})
	require.NoError(t, err)

	assert.Equal(t, uploaded, attachment.URI, "FileAPIURI が使われていません")
	assert.False(t, files.called(), "FileAPIURI があるのにアップロードが走っています")
}

// TestFileAPIResolverReusesUpload は、2回目以降がキャッシュから返り、
// アップロードが繰り返されないことを確認します。
func TestFileAPIResolverReusesUpload(t *testing.T) {
	files := &countingUploader{}
	r := newTestFileAPIResolver(t, FileAPIResolverConfig{Files: files})
	uri := ports.ImageURI{ReferenceURL: "https://example.com/char.png"}

	for i := range 3 {
		if _, err := r.Resolve(context.Background(), uri); err != nil {
			t.Fatalf("Resolve() #%d error = %v", i, err)
		}
	}
	if got := files.uploads.Load(); got != 1 {
		t.Errorf("uploads = %d, want 1", got)
	}
}

// TestFileAPIResolverDeduplicatesConcurrentUploads は、同一ソースへの同時呼び出しが
// 1回のアップロードにまとまることを確認します。キャッシュは完了後にしか書かれないため、
// singleflight が無いと並行呼び出しの数だけ File API 上に重複ファイルができます。
func TestFileAPIResolverDeduplicatesConcurrentUploads(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		files := &countingUploader{uploadDelay: 20 * time.Millisecond}
		r := newTestFileAPIResolver(t, FileAPIResolverConfig{Files: files})

		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				if _, err := r.Resolve(context.Background(), ports.ImageURI{ReferenceURL: "https://example.com/same.png"}); err != nil {
					t.Errorf("Resolve() error = %v", err)
				}
			})
		}
		wg.Wait()

		if got := files.uploads.Load(); got != 1 {
			t.Errorf("uploads = %d, want 1 (concurrent calls must share one upload)", got)
		}
	})
}

// TestFileAPIResolverDeclinesOnUploadFailure は、アップロードが失敗したときに
// 「管轄外」として次の resolver へ委ねることを確認します。アップロードは送信量を
// 減らす最適化なので、その失敗で生成そのものを落とす理由はありません。
func TestFileAPIResolverDeclinesOnUploadFailure(t *testing.T) {
	files := &countingUploader{uploadErr: errors.New("quota exceeded")}
	r := newTestFileAPIResolver(t, FileAPIResolverConfig{Files: files})

	_, err := r.Resolve(context.Background(), ports.ImageURI{ReferenceURL: "https://example.com/char.png"})
	if !errors.Is(err, ports.ErrResolverNotApplicable) {
		t.Fatalf("error = %v, want ErrResolverNotApplicable (次の経路へ委ねる)", err)
	}
}

// TestFileAPIResolverCancelDoesNotDecline は、呼び出し側のキャンセルが原因の
// アップロード失敗では辞退せずキャンセルを返すことを確認します。以前は
// 「アップロード失敗」と誤警告した上で同じ画像を再フェッチし、同じキャンセルで
// 落ちていました（無駄な二重フェッチ）。
func TestFileAPIResolverCancelDoesNotDecline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const uploadDelay = 50 * time.Millisecond
		files := &countingUploader{uploadDelay: uploadDelay}
		r := newTestFileAPIResolver(t, FileAPIResolverConfig{Files: files})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 呼び出し前にキャンセル済みにする

		_, err := r.Resolve(ctx, ports.ImageURI{ReferenceURL: "https://example.com/char.png"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled (辞退せず伝播する)", err)
		}

		// アップロードは呼び出し側のキャンセルでは止まらない設計（singleflight で
		// 相乗りしている他の呼び出しを巻き添えにしないため）なので、Resolve が
		// 返ったあとも走り続けます。バブルはその完了まで見届けてから閉じます。
		synctest.Sleep(uploadDelay)
		if got := files.uploads.Load(); got != 1 {
			t.Errorf("uploads = %d, want 1（キャンセルした呼び出しの裏で完走する）", got)
		}
	})
}

// TestFileAPIResolverCachesUploadedURI は、アップロード結果が素の URI 文字列として
// キャッシュされ、2回目がそれを使うことを確認します。
func TestFileAPIResolverCachesUploadedURI(t *testing.T) {
	cache := &mockCache{}
	files := &fakeUploader{}
	r := newTestFileAPIResolver(t, FileAPIResolverConfig{Files: files, Cache: cache})
	const src = "https://example.com/test.png"

	uri, err := r.ensureUploaded(context.Background(), src)
	require.NoError(t, err)
	assert.Equal(t, MockFileUploadURI, uri)

	cached, ok := cache.Get(cacheKeyFileAPI + src)
	require.True(t, ok, "should be cached")
	entry, ok := cached.(string)
	require.True(t, ok, "cache entry should be the uploaded URI string")
	assert.Equal(t, uri, entry)
}

// TestFileAPIResolverUsesDetectedMIMEType は、拡張子ではなく実データの MIME で
// アップロードすることを確認します。
func TestFileAPIResolverUsesDetectedMIMEType(t *testing.T) {
	files := &fakeUploader{}
	pngHeader := []byte("\x89PNG\r\n\x1a\n")
	r := newTestFileAPIResolver(t, FileAPIResolverConfig{
		Files:      files,
		Downloader: &mockHTTPClient{data: append(pngHeader, []byte("fake-image-binary")...)},
	})

	_, err := r.ensureUploaded(context.Background(), "https://example.com/no-extension")
	require.NoError(t, err)
	assert.Equal(t, "image/png", files.mimeType())
}

// TestFileAPIResolverCompressesBeforeUpload は、Compress 指定時に JPEG へ再圧縮して
// からアップロードすることを確認します。
func TestFileAPIResolverCompressesBeforeUpload(t *testing.T) {
	files := &fakeUploader{}
	r := newTestFileAPIResolver(t, FileAPIResolverConfig{Files: files, Compress: true})

	_, err := r.ensureUploaded(context.Background(), "https://example.com/input.png")
	require.NoError(t, err)

	assert.Equal(t, "image/jpeg", files.mimeType())
	_, format, err := image.Decode(bytes.NewReader(files.lastUploadData))
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
}

// TestCacheTTLIsPassedThroughUnchanged は、CacheTTL に既定値を差し込まないことを固定します。
// 0 は主要なキャッシュ実装で「キャッシュ側の既定に従う」という意味を持つため、ここで
// 値を作ると呼び出し側の設定を上書きしてしまいます。
func TestCacheTTLIsPassedThroughUnchanged(t *testing.T) {
	for _, ttl := range []time.Duration{0, 90 * time.Minute} {
		cache := &recordingCache{}
		r, err := NewFileAPIResolver(FileAPIResolverConfig{
			Files:      &fakeUploader{},
			Reader:     &mockReader{},
			Downloader: &mockHTTPClient{data: createPNGData(t)},
			Cache:      cache,
			CacheTTL:   ttl,
		})
		require.NoError(t, err)

		if _, err := r.ensureUploaded(context.Background(), "https://example.com/a.png"); err != nil {
			t.Fatalf("ensureUploaded() error = %v", err)
		}
		if cache.lastTTL != ttl {
			t.Errorf("TTL passed to the cache = %v, want %v (unchanged)", cache.lastTTL, ttl)
		}
	}
}

// recordingCache は Set に渡された TTL を記録するキャッシュです。
type recordingCache struct {
	mockCache
	lastTTL time.Duration
}

func (c *recordingCache) Set(key string, value any, ttl time.Duration) {
	c.lastTTL = ttl
	c.mockCache.Set(key, value, ttl)
}
