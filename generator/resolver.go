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

// backend は、resolver が成立するバックエンドの制約です。
type backend int

const (
	// backendAny はどちらのバックエンドでも使えることを表します。
	backendAny backend = iota
	// backendVertexAI は Vertex AI でしか成立しないことを表します（gs:// の直接参照）。
	backendVertexAI
	// backendGeminiAPI は Gemini API でしか成立しないことを表します（File API）。
	backendGeminiAPI
)

// backendConstrained は、特定のバックエンドでしか意味を持たない resolver が実装する
// パッケージ内部の面です。New が構築時に取り違えを弾くために使います。
//
// 非公開なのは、この判定がキット同梱の resolver の都合であり、利用側が自作した
// resolver に実装させたい契約ではないためです。
type backendConstrained interface {
	requiredBackend() backend
}

// requiredBackend は resolver のバックエンド制約を返します。制約を表明しない
// resolver（FetchResolver や利用側の自作実装）は backendAny です。
func requiredBackend(r ports.ReferenceResolver) backend {
	if c, ok := r.(backendConstrained); ok {
		return c.requiredBackend()
	}
	return backendAny
}

// ResolverChain は複数の resolver を順に試します。
//
// 次へ進む合図は ports.ErrResolverNotApplicable（管轄外）だけで、それ以外のエラーは
// その場で返します。誰も扱えなければ ErrUnresolvedReference になります。
//
// FetchResolver は最後段に置いてください。FileAPIResolver はアップロードに失敗すると
// 辞退して次の resolver に委ねるため（FileAPIResolver.Resolve を参照）、後ろに受け皿が
// 無いとその辞退が行き先を失い、生成が ErrUnresolvedReference で止まります。
type ResolverChain struct {
	resolvers []ports.ReferenceResolver
}

// NewResolverChain は resolver を並べた解決経路を作ります。並び順が優先順です。
// 典型的な組み合わせは New のドキュメントを参照してください。
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

// requiredBackend は、連なる resolver のうち最初に見つかった制約を返します。
//
// 矛盾する組み合わせ（gs:// 直参照 + File API）を並べた場合、ここでは最初のものしか
// 見ませんが、実クライアントはどちらか一方のバックエンドなので、New では必ず
// どちらかの制約が食い違って弾かれます。
func (c *ResolverChain) requiredBackend() backend {
	for _, r := range c.resolvers {
		if b := requiredBackend(r); b != backendAny {
			return b
		}
	}
	return backendAny
}

// GCSResolver は gs:// URI を、バイト列を転送せずそのまま参照させます。
//
// Vertex AI 専用です。Vertex AI は gs:// をモデル側で解決できますが、Gemini API
// バックエンドはできないため、そちらへ渡すと生成時に不可解な失敗をします。New は
// 構築時に ErrVertexAIRequired で弾きます。
//
// 依存を一切取りません。取得も、アップロードも、キャッシュもしないためです。
// 参照が gs:// だけで済む構成なら、これ 1 つで足ります。
type GCSResolver struct{}

// NewGCSResolver は GCSResolver を作ります。設定項目はありません。
func NewGCSResolver() *GCSResolver { return &GCSResolver{} }

func (r *GCSResolver) requiredBackend() backend { return backendVertexAI }

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
	// どちらも必須です（取得先の検証責務は ports.Downloader を参照）。
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
// MIME type は mimeHintURI の拡張子から推測します。判別できない場合は設定しません
// （理由は imgutil.MIMETypeByPath を参照）。
func fileAttachment(fileURI, mimeHintURI string) gemini.Attachment {
	return gemini.Attachment{URI: fileURI, MIMEType: imgutil.MIMETypeByPath(mimeHintURI)}
}
