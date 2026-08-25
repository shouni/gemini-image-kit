package generator

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"golang.org/x/sync/singleflight"

	"github.com/shouni/gemini-image-kit/internal/imgutil"
	"github.com/shouni/gemini-image-kit/ports"
)

const cacheKeyFileAPI = "fileapi:"

// FileAPIResolverConfig は FileAPIResolver の依存と設定です。
//
// 位置引数で受けると、呼び出し側に true/false や数値が並んで何の設定か読めなくなります。
type FileAPIResolverConfig struct {
	// Files は File API へのアップロード先です（*gemini.Client が満たします）。必須。
	Files gemini.FileManager
	// Reader は gs:// の取得に、Downloader は http(s):// の取得に使います。
	// アップロードするにはまず取得が要るため、どちらも必須です
	// （取得先の検証責務は ports.Downloader を参照）。
	Reader     ports.ContentReader
	Downloader ports.Downloader
	// Cache はアップロード済み URI の再利用先です。必須。
	//
	// 必須なのは、アップロードの使い回しがこの resolver の存在理由そのものであり、
	// キャッシュ無しでは同じ参照画像を毎回上げ直すだけになるためです。キャッシュを
	// 持ちたくない構成では、この resolver ではなく FetchResolver を使ってください。
	Cache ports.ImageCacher
	// CacheTTL はアップロード済みファイルの参照を保持する期間です。
	//
	// **0 は補完しません。** 利用側が使っている ttlcache では 0 が `ttlcache.DefaultTTL`
	// そのもので、「キャッシュ側に設定した既定の有効期間に従う」という意味を持ちます。
	// ここで既定値を差し込むと、呼び出し側のキャッシュ設定を上書きしてしまいます。
	//
	// 注意: File API 上のファイルにはサーバー側の保持期限があります。実装側の既定が
	// 「無期限」や保持期限より長い設定になっていると、失効した URI を参照し続けて
	// 生成が失敗します。保持期限より短い値を明示するのが安全です。
	CacheTTL time.Duration
	// UploadTimeout はアップロード1回あたりの制限時間です。0 以下なら DefaultUploadTimeout。
	//
	// 同一ソースへの同時アップロードは1回にまとめられ、その共有実行は呼び出し元の
	// context から切り離されるため（先に離脱した呼び出し元が他を巻き添えにしないため）、
	// 共有実行を打ち切れるのはこのタイムアウトだけです。
	UploadTimeout time.Duration
	// FetchTimeout は取得1回あたりの制限時間です。0 以下なら DefaultFetchTimeout。
	FetchTimeout time.Duration
	// MaxReferenceBytes は参照画像1枚あたりのサイズ上限です。
	// 0 以下なら DefaultMaxReferenceBytes。
	MaxReferenceBytes int64
	// Compress を true にすると PNG/GIF を JPEG へ再圧縮してからアップロードします。
	Compress bool
	// CompressionQuality は Compress が true のときの JPEG 品質です。
	// 0 以下なら DefaultCompressionQuality。
	CompressionQuality int
	// Logger は、この resolver が出すログの出力先です。nil なら slog.Default()。
	Logger *slog.Logger
}

// FileAPIResolver は参照画像を Gemini File API へアップロードし、以降は URI で参照します。
//
// **Gemini API バックエンド専用です。** Vertex AI に File API はありません。
//
// 同じ参照画像を繰り返し使うワークロード（同じキャラクターを何枚もの生成で使う等）では、
// 毎回バイト列を送るより安く済みます。逆に参照が毎回異なる使い捨てのワークロードでは、
// アップロードの往復と File API 上のファイルが無駄になるため FetchResolver が向きます。
type FileAPIResolver struct {
	files       gemini.FileManager
	fetcher     sourceFetcher
	compression compression
	cache       ports.ImageCacher
	expiration  time.Duration
	timeout     time.Duration
	logger      *slog.Logger
	// uploadGroup は同一ソースの同時アップロードを1回にまとめます。
	uploadGroup singleflight.Group
}

func (r *FileAPIResolver) requiredBackend() backend { return backendGeminiAPI }

// NewFileAPIResolver はアップロード + キャッシュで参照を解決する resolver を作ります。
func NewFileAPIResolver(cfg FileAPIResolverConfig) (*FileAPIResolver, error) {
	if cfg.Files == nil {
		return nil, ErrFileManagerRequired
	}
	if cfg.Cache == nil {
		return nil, ErrCacheRequired
	}
	fetcher, err := newSourceFetcher(cfg.Reader, cfg.Downloader, cfg.FetchTimeout, cfg.MaxReferenceBytes)
	if err != nil {
		return nil, err
	}
	uploadTimeout := cfg.UploadTimeout
	if uploadTimeout <= 0 {
		uploadTimeout = DefaultUploadTimeout
	}

	return &FileAPIResolver{
		files:       cfg.Files,
		fetcher:     fetcher,
		compression: newCompression(cfg.Compress, cfg.CompressionQuality),
		cache:       cfg.Cache,
		expiration:  cfg.CacheTTL,
		timeout:     uploadTimeout,
		logger:      cfg.Logger,
	}, nil
}

// Resolve は参照を File API 上の URI 参照へ解決します。
//
// 呼び出し側が既にアップロード済みの URI（ImageURI.FileAPIURI）を持っていれば、
// アップロードを省いてそれを使います。
//
// アップロードに失敗した場合は辞退（ErrResolverNotApplicable）して次の resolver に
// 委ねます。アップロードは送信量を減らすための最適化であって正しさの要件ではなく、
// 取得そのものが失敗しているなら後段でも同じ理由で落ちるためです。
//
// ただし失敗の原因が呼び出し側のキャンセルである場合は辞退せずエラーを返します。
// 辞退すると、同じ画像をもう一度フェッチして同じキャンセルで落ちるだけだからです。
func (r *FileAPIResolver) Resolve(ctx context.Context, uri ports.ImageURI) (gemini.Attachment, error) {
	if uri.FileAPIURI != "" {
		return fileAttachment(uri.FileAPIURI, uri.ReferenceURL), nil
	}
	if uri.ReferenceURL == "" {
		return gemini.Attachment{}, ports.ErrResolverNotApplicable
	}

	uploadedURI, err := r.ensureUploaded(ctx, uri.ReferenceURL)
	if err != nil {
		if ctx.Err() != nil {
			return gemini.Attachment{}, ctx.Err()
		}
		r.log().WarnContext(ctx, "参照画像の File API へのアップロードに失敗しました。次の解決経路に委ねます",
			"reference", uri.ReferenceURL, "error", err)
		return gemini.Attachment{}, ports.ErrResolverNotApplicable
	}
	return fileAttachment(uploadedURI, uri.ReferenceURL), nil
}

// ensureUploaded は画像を File API にアップロードし、アップロード先の URI を返します。
// すでにアップロード済みならキャッシュの URI を返します。
func (r *FileAPIResolver) ensureUploaded(ctx context.Context, fileURI string) (string, error) {
	if uri, ok := r.lookupCache(fileURI); ok {
		return uri, nil
	}
	return r.uploadOnce(ctx, fileURI)
}

// uploadOnce は、同一ソースへの同時アップロードを1回にまとめます。
//
// キャッシュは完了後にしか書かれないため、これが無いと同じ参照画像を並行して
// 使う呼び出し（複数参照の生成や、同じキャラクターを使う複数ジョブ）がそれぞれ
// アップロードし、File API 上に重複ファイルを作ります。
//
// 共有実行は呼び出し元の context から切り離します。最初の呼び出し元がキャンセルした
// だけで、相乗りしている他の呼び出し元まで巻き添えになるのを避けるためです。
// 打ち切りは UploadTimeout が担います。
func (r *FileAPIResolver) uploadOnce(ctx context.Context, fileURI string) (string, error) {
	ch := r.uploadGroup.DoChan(fileURI, func() (any, error) {
		execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.timeout)
		defer cancel()

		// 直前に別の実行が完了している可能性があるため、もう一度キャッシュを見る。
		if uri, ok := r.lookupCache(fileURI); ok {
			return uri, nil
		}
		return r.fetchAndUpload(execCtx, fileURI)
	})

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-ch:
		if result.Err != nil {
			return "", result.Err
		}
		uri, _ := result.Val.(string)
		return uri, nil
	}
}

// fetchAndUpload はソースを取得して File API へアップロードし、結果をキャッシュします。
func (r *FileAPIResolver) fetchAndUpload(ctx context.Context, fileURI string) (string, error) {
	rc, err := r.fetcher.open(ctx, fileURI)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	br := bufio.NewReader(rc)
	mimeType, err := detectUploadSource(br)
	if err != nil {
		return "", err
	}

	uploaded, err := r.upload(ctx, br, mimeType, fileURI)
	if err != nil {
		return "", err
	}

	r.storeCache(fileURI, uploaded.URI)
	return uploaded.URI, nil
}

// upload は圧縮設定に基づいてアップロードを実行します。
//
// 圧縮対象でない形式はデコードせず、取得元の bufio.Reader をそのまま渡します
// （デコード・再エンコードのコストを掛けません）。
func (r *FileAPIResolver) upload(ctx context.Context, src io.Reader, mimeType, fileURI string) (gemini.UploadedFile, error) {
	if !r.compression.applies(mimeType) {
		return r.files.UploadFile(ctx, src, mimeType, uploadDisplayName(fileURI))
	}

	compressed, err := imgutil.CompressToJPEG(src, r.compression.quality)
	if err != nil {
		return gemini.UploadedFile{}, fmt.Errorf("failed to compress image for upload: %w", err)
	}
	return r.files.UploadFile(ctx, bytes.NewReader(compressed), "image/jpeg", uploadDisplayName(fileURI))
}

// lookupCache は、ソース URI に紐づくアップロード済み URI を取得します。
// 文字列以外が入っていた場合はミス扱いにするため、キャッシュを共有していて
// 別形式の値が混ざっても壊れません。
func (r *FileAPIResolver) lookupCache(sourceURI string) (string, bool) {
	val, ok := r.cache.Get(cacheKeyFileAPI + sourceURI)
	if !ok {
		return "", false
	}
	uri, ok := val.(string)
	if !ok || uri == "" {
		return "", false
	}
	return uri, true
}

func (r *FileAPIResolver) storeCache(sourceURI string, uploadedURI string) {
	r.cache.Set(cacheKeyFileAPI+sourceURI, uploadedURI, r.expiration)
}

func (r *FileAPIResolver) log() *slog.Logger {
	if r.logger != nil {
		return r.logger
	}
	return slog.Default()
}
