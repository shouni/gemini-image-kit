package generator

import (
	"context"
	"errors"
	"testing"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

func newTestFetchResolver(t *testing.T, cfg FetchResolverConfig) *FetchResolver {
	t.Helper()
	png := createPNGData(t)
	if cfg.Reader == nil {
		cfg.Reader = &mockReader{data: png}
	}
	if cfg.Downloader == nil {
		cfg.Downloader = &mockHTTPClient{data: png}
	}
	r, err := NewFetchResolver(cfg)
	if err != nil {
		t.Fatalf("NewFetchResolver() error = %v", err)
	}
	return r
}

// TestGCSResolverPassesThroughGCSURI は、gs:// が転送なしの直接参照になることを
// 確認します（最も安い経路です）。
func TestGCSResolverPassesThroughGCSURI(t *testing.T) {
	attachment, err := NewGCSResolver().Resolve(context.Background(),
		ports.ImageURI{ReferenceURL: "gs://bucket/char.png"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if attachment.URI != "gs://bucket/char.png" {
		t.Errorf("URI = %q, want the gs:// reference", attachment.URI)
	}
	if attachment.MIMEType != "image/png" {
		t.Errorf("MIMEType = %q, want image/png", attachment.MIMEType)
	}
	if len(attachment.Data) != 0 {
		t.Error("URI 参照なのにバイト列が載っています")
	}
}

// TestGCSResolverDeclinesNonGCS は、gs:// 以外を「失敗」ではなく「管轄外」として
// 返すことを確認します。これがチェーンで次へ進む合図になります。
func TestGCSResolverDeclinesNonGCS(t *testing.T) {
	_, err := NewGCSResolver().Resolve(context.Background(),
		ports.ImageURI{ReferenceURL: "https://example.com/char.png"})
	if !errors.Is(err, ports.ErrResolverNotApplicable) {
		t.Fatalf("error = %v, want ErrResolverNotApplicable", err)
	}
}

// TestFetchResolverInlinesBytes は、取得したバイト列がインライン添付になることを
// 確認します。
func TestFetchResolverInlinesBytes(t *testing.T) {
	r := newTestFetchResolver(t, FetchResolverConfig{})

	attachment, err := r.Resolve(context.Background(), ports.ImageURI{ReferenceURL: "https://example.com/char.png"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(attachment.Data) == 0 {
		t.Error("インラインのバイト列が載っていません")
	}
	if attachment.MIMEType != "image/png" {
		t.Errorf("MIMEType = %q, want image/png", attachment.MIMEType)
	}
}

// TestFetchResolverRejectsNonImage は、画像として扱えないデータをエラーにすることを
// 確認します。
func TestFetchResolverRejectsNonImage(t *testing.T) {
	r := newTestFetchResolver(t, FetchResolverConfig{
		Downloader: &mockHTTPClient{data: []byte("not an image at all")},
	})

	attachment, err := r.Resolve(context.Background(), ports.ImageURI{ReferenceURL: "https://example.com/x.png"})
	if !errors.Is(err, ErrUnsupportedFileFormat) {
		t.Fatalf("error = %v, want ErrUnsupportedFileFormat", err)
	}
	if !attachment.IsEmpty() {
		t.Error("エラー時に添付が返っています")
	}
}

// TestFetchResolverPropagatesFetchError は、取得失敗がそのままエラーになることを
// 確認します（管轄外ではないので、チェーンは次へ進んではいけません）。
func TestFetchResolverPropagatesFetchError(t *testing.T) {
	fetchErr := errors.New("network down")
	r := newTestFetchResolver(t, FetchResolverConfig{Downloader: &mockHTTPClient{err: fetchErr}})

	_, err := r.Resolve(context.Background(), ports.ImageURI{ReferenceURL: "https://example.com/x.png"})
	if !errors.Is(err, fetchErr) {
		t.Fatalf("error = %v, want the fetch error", err)
	}
	if errors.Is(err, ports.ErrResolverNotApplicable) {
		t.Error("取得失敗を管轄外として扱ってはいけません（障害が別経路に紛れます）")
	}
}

// TestFetchResolverEnforcesSizeLimit は、MaxReferenceBytes を超える参照が
// エラーになることを確認します。上限なしの読み込みはメモリと時間の危険です。
func TestFetchResolverEnforcesSizeLimit(t *testing.T) {
	r := newTestFetchResolver(t, FetchResolverConfig{MaxReferenceBytes: 8})

	_, err := r.Resolve(context.Background(), ports.ImageURI{ReferenceURL: "https://example.com/big.png"})
	if !errors.Is(err, ErrReferenceTooLarge) {
		t.Fatalf("error = %v, want ErrReferenceTooLarge", err)
	}
}

// TestFetchResolverRequiresDependencies は、取得に必要な依存の nil チェックを検証します。
func TestFetchResolverRequiresDependencies(t *testing.T) {
	tests := []struct {
		name    string
		cfg     FetchResolverConfig
		wantErr error
	}{
		{"Reader なし", FetchResolverConfig{Downloader: &mockHTTPClient{}}, ErrReaderRequired},
		{"Downloader なし", FetchResolverConfig{Reader: &mockReader{}}, ErrHTTPClientRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewFetchResolver(tt.cfg)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if r != nil {
				t.Error("エラー時に resolver が返っています")
			}
		})
	}
}

// TestResolverChainFallsThroughOnNotApplicable は、管轄外の resolver を飛ばして
// 次が使われることを確認します（Vertex の典型構成: gs:// は直接、他は取得）。
func TestResolverChainFallsThroughOnNotApplicable(t *testing.T) {
	chain := NewResolverChain(NewGCSResolver(), newTestFetchResolver(t, FetchResolverConfig{}))

	gcs, err := chain.Resolve(context.Background(), ports.ImageURI{ReferenceURL: "gs://bucket/a.png"})
	if err != nil {
		t.Fatalf("Resolve(gs://) error = %v", err)
	}
	if gcs.URI == "" || len(gcs.Data) != 0 {
		t.Errorf("gs:// は URI 参照であるべきです: %+v", gcs)
	}

	http, err := chain.Resolve(context.Background(), ports.ImageURI{ReferenceURL: "https://example.com/a.png"})
	if err != nil {
		t.Fatalf("Resolve(https://) error = %v", err)
	}
	if len(http.Data) == 0 {
		t.Error("http はインライン送信であるべきです")
	}
}

// TestResolverChainStopsOnRealError は、管轄外以外のエラーでチェーンが止まることを
// 確認します。取得失敗で次へ流すと、ネットワーク障害が「解決できません」に
// すり替わって原因が消えます。
func TestResolverChainStopsOnRealError(t *testing.T) {
	boom := errors.New("network down")
	failing := &stubResolver{resolve: func(context.Context, string) (gemini.Attachment, error) {
		return gemini.Attachment{}, boom
	}}
	reached := false
	next := &stubResolver{resolve: func(context.Context, string) (gemini.Attachment, error) {
		reached = true
		return gemini.Attachment{MIMEType: "image/png", Data: []byte("x")}, nil
	}}

	_, err := NewResolverChain(failing, next).Resolve(context.Background(),
		ports.ImageURI{ReferenceURL: "https://example.com/a.png"})
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want the underlying failure", err)
	}
	if reached {
		t.Error("実エラーなのに次の resolver へ進んでいます")
	}
}

// TestResolverChainReportsUnresolved は、誰も扱えなかった参照が経路の設定漏れとして
// 分かるエラーになることを確認します。
func TestResolverChainReportsUnresolved(t *testing.T) {
	_, err := NewResolverChain(NewGCSResolver()).Resolve(context.Background(),
		ports.ImageURI{ReferenceURL: "https://example.com/a.png"})
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("error = %v, want ErrUnresolvedReference", err)
	}
}

// TestResolverChainInheritsBackendConstraint は、制約付き resolver を含むチェーンが
// その制約を引き継ぐことを確認します。制約が消えると、バックエンドの取り違えを
// 構築時に弾けなくなります。
func TestResolverChainInheritsBackendConstraint(t *testing.T) {
	fetch := newTestFetchResolver(t, FetchResolverConfig{})

	tests := []struct {
		name  string
		chain *ResolverChain
		want  backend
	}{
		{"GCS を含む", NewResolverChain(NewGCSResolver(), fetch), backendVertexAI},
		{"File API を含む", NewResolverChain(newTestFileAPIResolver(t, FileAPIResolverConfig{}), fetch), backendGeminiAPI},
		{"取得のみ", NewResolverChain(fetch), backendAny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chain.requiredBackend(); got != tt.want {
				t.Errorf("requiredBackend() = %v, want %v", got, tt.want)
			}
		})
	}
}
