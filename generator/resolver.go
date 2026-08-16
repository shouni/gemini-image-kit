package generator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/internal/imgutil"
	"github.com/shouni/gemini-image-kit/ports"
)

// vertexOnlyResolver は、Vertex AI バックエンドでしか意味を持たない resolver が
// 実装するパッケージ内部の面です。New が構築時に取り違えを弾くために使います。
//
// 非公開なのは、この判定がキット同梱の resolver の都合であり、利用側が自作した
// resolver に実装させたい契約ではないためです。
type vertexOnlyResolver interface {
	requiresVertexAI() bool
}

// ResolverChain は複数の resolver を順に試します。
//
// ports.ErrResolverNotApplicable（管轄外）だけが次への合図です。それ以外のエラーは
// その場で返します — 取得失敗で次へ流すと、ネットワーク障害が「参照を解決できません」に
// すり替わって原因が消えるためです。
type ResolverChain struct {
	resolvers []ports.ReferenceResolver
}

// NewResolverChain は resolver を並べた解決経路を作ります。
//
// 並び順が優先順です。典型的な構成は次の 2 つです。
//
//	// Vertex AI: gs:// はそのまま参照し（転送ゼロ）、それ以外は取得してインライン
//	NewResolverChain(NewGCSResolver(), fetchResolver)
//
//	// Gemini API: File API へ上げて URI 参照し、失敗したら取得してインライン
//	NewResolverChain(fileAPIResolver, fetchResolver)
func NewResolverChain(resolvers ...ports.ReferenceResolver) *ResolverChain {
	return &ResolverChain{resolvers: resolvers}
}

// Resolve は、参照を扱える最初の resolver に解決させます。
func (c *ResolverChain) Resolve(ctx context.Context, uri ports.ImageURI) (gemini.Attachment, error) {
	for _, r := range c.resolvers {
		attachment, err := r.Resolve(ctx, uri)
		if errors.Is(err, ports.ErrResolverNotApplicable) {
			continue
		}
		return attachment, err
	}
	return gemini.Attachment{}, fmt.Errorf("%w: %q", ErrUnresolvedReference, uri.ReferenceURL)
}

// requiresVertexAI は、連なる resolver のいずれかが Vertex AI 専用なら true を返します。
func (c *ResolverChain) requiresVertexAI() bool {
	for _, r := range c.resolvers {
		if v, ok := r.(vertexOnlyResolver); ok && v.requiresVertexAI() {
			return true
		}
	}
	return false
}

// GCSResolver は gs:// URI を、バイト列を転送せずそのまま参照させます。
//
// **Vertex AI 専用です。** Vertex AI は gs:// をモデル側で解決できますが、Gemini API
// バックエンドはできないため、そちらへ渡すと生成時に不可解な失敗をします。New は
// 構築時に ErrVertexAIRequired で弾きます。
//
// 依存を一切取りません。取得も、アップロードも、キャッシュもしないためです。
// 参照が gs:// だけで済む構成なら、これ 1 つで足ります。
type GCSResolver struct{}

// NewGCSResolver は gs:// 参照をそのまま渡す resolver を作ります（Vertex AI 専用）。
func NewGCSResolver() *GCSResolver { return &GCSResolver{} }

func (r *GCSResolver) requiresVertexAI() bool { return true }

// Resolve は gs:// URI を URI 参照の添付にします。gs:// 以外は辞退します。
func (r *GCSResolver) Resolve(_ context.Context, uri ports.ImageURI) (gemini.Attachment, error) {
	if !isGCSURI(uri.ReferenceURL) {
		return gemini.Attachment{}, ports.ErrResolverNotApplicable
	}
	return fileAttachment(uri.ReferenceURL, uri.ReferenceURL), nil
}

// FetchResolverConfig は FetchResolver の依存と設定です。
type FetchResolverConfig struct {
	// Reader は gs:// の取得に、Downloader は http(s):// の取得に使います。
	// どちらも必須です。
	//
	// 注意: Downloader には呼び出し側が任意入力した URL がそのまま渡ります。
	// SSRF 対策とドメイン許可リストは意図的に呼び出し側の責務です。
	Reader     ports.ContentReader
	Downloader ports.Downloader
	// FetchTimeout は取得1回あたりの制限時間です。0 以下なら DefaultFetchTimeout。
	FetchTimeout time.Duration
	// MaxReferenceBytes は参照画像1枚あたりのサイズ上限です。
	// 0 以下なら DefaultMaxReferenceBytes。
	MaxReferenceBytes int64
	// Compress を true にすると PNG/GIF を JPEG へ再圧縮してから送ります。
	Compress bool
	// CompressionQuality は Compress が true のときの JPEG 品質です。
	// 0 以下なら DefaultCompressionQuality。
	CompressionQuality int
}

// FetchResolver は参照画像を取得し、バイト列としてインライン送信します。
//
// どのバックエンドでも使えます。gs:// も http(s):// も扱えるため、経路の最後段
// （どの resolver も扱えなかったときの受け皿）に置くのが基本です。
type FetchResolver struct {
	fetcher     sourceFetcher
	compression compression
}

// NewFetchResolver は取得してインライン送信する resolver を作ります。
func NewFetchResolver(cfg FetchResolverConfig) (*FetchResolver, error) {
	fetcher, err := newSourceFetcher(cfg.Reader, cfg.Downloader, cfg.FetchTimeout, cfg.MaxReferenceBytes)
	if err != nil {
		return nil, err
	}
	return &FetchResolver{
		fetcher:     fetcher,
		compression: newCompression(cfg.Compress, cfg.CompressionQuality),
	}, nil
}

// Resolve は参照画像を取得し、インライン添付にします。
func (r *FetchResolver) Resolve(ctx context.Context, uri ports.ImageURI) (gemini.Attachment, error) {
	rawData, err := r.fetcher.readAll(ctx, uri.ReferenceURL)
	if err != nil {
		return gemini.Attachment{}, err
	}

	mimeType := imgutil.DetectMIMEType(rawData)
	if !imgutil.IsImageMIMEType(mimeType) {
		return gemini.Attachment{}, fmt.Errorf("%w: %s", ErrUnsupportedFileFormat, mimeType)
	}

	if r.compression.applies(mimeType) {
		compressed, err := imgutil.CompressToJPEG(bytes.NewReader(rawData), r.compression.quality)
		if err != nil {
			return gemini.Attachment{}, fmt.Errorf("failed to compress image: %w", err)
		}
		rawData = compressed
		mimeType = "image/jpeg"
	}

	return gemini.Attachment{MIMEType: mimeType, Data: rawData}, nil
}

// fileAttachment は URI 参照の添付を生成します。
//
// 拡張子から MIME type を判別できない場合は MIMEType を設定しません。
// 誤った型を申告するとサーバー側のデコードが失敗しうるため、
// 推測できないときはサーバーのコンテンツ判定に委ねます。
func fileAttachment(fileURI, mimeHintURI string) gemini.Attachment {
	return gemini.Attachment{URI: fileURI, MIMEType: imgutil.GuessMIMEType(mimeHintURI)}
}
