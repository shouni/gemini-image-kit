// Package generator は、Gemini API を用いた画像生成・編集の実行と、
// File API へのアップロード・キャッシュ管理を行う基盤ロジックを提供します。
package generator

import (
	"log/slog"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"golang.org/x/sync/singleflight"

	"github.com/shouni/gemini-image-kit/ports"
)

const (
	// DefaultCompressionQuality は、圧縮品質が指定されなかった場合のJPEG品質値です。
	DefaultCompressionQuality = 75
	// DefaultUploadTimeout は、UploadTimeout が指定されなかった場合の
	// アップロード1回あたりの制限時間です。
	DefaultUploadTimeout = 2 * time.Minute
	// DefaultFetchTimeout は、FetchTimeout が指定されなかった場合の
	// 参照画像取得1回あたりの制限時間です。
	DefaultFetchTimeout = time.Minute
	// DefaultMaxReferenceBytes は、MaxReferenceBytes が指定されなかった場合の
	// 参照画像1枚あたりのサイズ上限です。
	DefaultMaxReferenceBytes int64 = 32 << 20 // 32 MiB
	cacheKeyFileAPI                = "fileapi:"
)

// GeminiImageCore は参照画像の解決（取得・アップロード・キャッシュ）と
// 生成リクエストの実行を担う基盤クラスです。
type GeminiImageCore struct {
	aiClient           gemini.Model
	reader             ports.ContentReader
	httpClient         ports.Downloader
	cache              ports.ImageCacher
	expiration         time.Duration
	compress           bool
	compressionQuality int
	inlineReferences   bool
	uploadTimeout      time.Duration
	fetchTimeout       time.Duration
	maxReferenceBytes  int64
	logger             *slog.Logger
	// uploadGroup は同一ソースの同時アップロードを1回にまとめます。
	uploadGroup singleflight.Group
}

// GeminiImageCoreConfig は GeminiImageCore の依存関係と設定です。
//
// 位置引数で受けると、呼び出し側に true/false や数値が並んで何の設定か読めなくなります。
type GeminiImageCoreConfig struct {
	// AIClient / Reader / HTTPClient / Cache はいずれも必須です。
	// Cache が必須なのは、アップロード済み参照の使い回しがこのキットの主要な
	// コスト最適化であり、キャッシュ無しでは毎回アップロードし直すためです。
	AIClient   gemini.Model
	Reader     ports.ContentReader
	HTTPClient ports.Downloader
	Cache      ports.ImageCacher
	// CacheTTL はアップロード済みファイルの参照を保持する期間です。
	//
	// **0 は補完しません。** 利用側が使っている ttlcache では 0 が `ttlcache.DefaultTTL`
	// そのもので、「キャッシュ側に設定した既定の有効期間に従う」という意味を持ちます。
	// ここで既定値を差し込むと、呼び出し側のキャッシュ設定を上書きしてしまいます。
	// CompressionQuality や UploadTimeout を補完するのは、それらがキット自身が使う値で
	// あって、委ねる先が無いためです。
	//
	// 注意: File API 上のファイルにはサーバー側の保持期限があります。実装側の既定が
	// 「無期限」や保持期限より長い設定になっていると、失効した files/... の URI を
	// 参照し続けて生成が失敗します。保持期限より短い値を明示するのが安全です。
	CacheTTL time.Duration
	// Compress を true にすると、参照画像を送信前に JPEG へ再圧縮します。
	Compress bool
	// InlineReferences を true にすると、参照画像を File API へ上げずに毎回
	// バイト列としてインライン送信します（resolveReference の解決方法を変えます）。
	//
	// 既定（false）では Gemini API バックエンドで File API へアップロードし、以降は
	// URI 参照で使い回します。同じ参照画像を繰り返し使うワークロードではこちらが有利です。
	// 逆に参照画像が毎回異なる（使い捨て）ワークロードでは、アップロードの往復と
	// File API 上のファイルが無駄になるため、true が向きます。
	InlineReferences bool
	// CompressionQuality は Compress が true のときの JPEG 品質です。
	// 0 以下なら DefaultCompressionQuality を使います。
	CompressionQuality int
	// UploadTimeout はアップロード1回あたりの制限時間です。0 以下なら
	// DefaultUploadTimeout を使います。
	//
	// 同一ソースへの同時アップロードは1回にまとめられ、その共有実行は呼び出し元の
	// context から切り離されるため（先に離脱した呼び出し元が他を巻き添えにしないため）、
	// 共有実行を打ち切れるのはこのタイムアウトだけです。
	UploadTimeout time.Duration
	// FetchTimeout は参照画像の取得（インライン送信経路）1回あたりの制限時間です。
	// 0 以下なら DefaultFetchTimeout を使います。
	FetchTimeout time.Duration
	// MaxReferenceBytes は参照画像1枚あたりのサイズ上限です。0 以下なら
	// DefaultMaxReferenceBytes を使います。上限を超えた参照はエラーになります
	// （無制限だと、巨大なリモートファイルをメモリへ丸ごと読み込んでしまいます）。
	MaxReferenceBytes int64
	// Logger は、このコアが出すログの出力先です。nil の場合は slog.Default() を使います。
	Logger *slog.Logger
}

// NewGeminiImageCore は依存関係を注入して GeminiImageCore を初期化します。
func NewGeminiImageCore(cfg GeminiImageCoreConfig) (*GeminiImageCore, error) {
	if cfg.AIClient == nil {
		return nil, ErrAIClientRequired
	}
	if cfg.Reader == nil {
		return nil, ErrReaderRequired
	}
	if cfg.HTTPClient == nil {
		return nil, ErrHTTPClientRequired
	}
	if cfg.Cache == nil {
		return nil, ErrCacheRequired
	}

	quality := cfg.CompressionQuality
	if quality <= 0 {
		quality = DefaultCompressionQuality
	}
	uploadTimeout := cfg.UploadTimeout
	if uploadTimeout <= 0 {
		uploadTimeout = DefaultUploadTimeout
	}
	fetchTimeout := cfg.FetchTimeout
	if fetchTimeout <= 0 {
		fetchTimeout = DefaultFetchTimeout
	}
	maxReferenceBytes := cfg.MaxReferenceBytes
	if maxReferenceBytes <= 0 {
		maxReferenceBytes = DefaultMaxReferenceBytes
	}

	return &GeminiImageCore{
		aiClient:           cfg.AIClient,
		reader:             cfg.Reader,
		httpClient:         cfg.HTTPClient,
		cache:              cfg.Cache,
		expiration:         cfg.CacheTTL,
		compress:           cfg.Compress,
		compressionQuality: quality,
		inlineReferences:   cfg.InlineReferences,
		uploadTimeout:      uploadTimeout,
		fetchTimeout:       fetchTimeout,
		maxReferenceBytes:  maxReferenceBytes,
		logger:             cfg.Logger,
	}, nil
}

// IsVertexAI は、Vertex AI バックエンドを使用しているかを確認します。
func (c *GeminiImageCore) IsVertexAI() bool {
	return c.aiClient.IsVertexAI()
}

// log は設定済みロガーを返します。構造体リテラルで直接構築するテストでも
// 安全に動くようフォールバックを持ちます。
func (c *GeminiImageCore) log() *slog.Logger {
	if c.logger != nil {
		return c.logger
	}
	return slog.Default()
}
