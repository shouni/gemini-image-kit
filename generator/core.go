// Package generator は、Gemini API を用いた画像生成・編集の実行と、
// File API へのアップロード・キャッシュ管理を行う基盤ロジックを提供します。
package generator

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/imgutil"
	"github.com/shouni/gemini-image-kit/ports"
)

const (
	// DefaultCompressionQuality は、圧縮品質が指定されなかった場合のJPEG品質値です。
	DefaultCompressionQuality = 75
	cacheKeyFileAPI           = "fileapi:"
)

// GeminiImageCore は AssetManager と ImageExecutor の両方の責務を担う基盤クラスです。
type GeminiImageCore struct {
	aiClient           gemini.MultimodalModel
	reader             ports.ContentReader
	httpClient         ports.Downloader
	cache              ports.ImageCacher
	expiration         time.Duration
	compress           bool
	compressionQuality int
}

// GeminiImageCoreConfig は GeminiImageCore の依存関係と設定です。
//
// 位置引数で受けると、呼び出し側に true/false や数値が並んで何の設定か読めなくなります。
type GeminiImageCoreConfig struct {
	// AIClient / Reader / HTTPClient / Cache はいずれも必須です。
	// Cache が必須なのは、DeleteFile が File API 上のファイル名をキャッシュから引くためです。
	AIClient   gemini.MultimodalModel
	Reader     ports.ContentReader
	HTTPClient ports.Downloader
	Cache      ports.ImageCacher
	// CacheTTL はアップロード済みファイルの参照を保持する期間です。
	CacheTTL time.Duration
	// Compress を true にすると、参照画像を送信前に JPEG へ再圧縮します。
	Compress bool
	// CompressionQuality は Compress が true のときの JPEG 品質です。
	// 0 以下なら DefaultCompressionQuality を使います。
	CompressionQuality int
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
	// cache は任意ではありません。DeleteFile が File API 上のファイル名を
	// キャッシュから引くため、nil だと削除が一切できなくなります。
	if cfg.Cache == nil {
		return nil, ErrCacheRequired
	}

	quality := cfg.CompressionQuality
	if quality <= 0 {
		quality = DefaultCompressionQuality
	}

	return &GeminiImageCore{
		aiClient:           cfg.AIClient,
		reader:             cfg.Reader,
		httpClient:         cfg.HTTPClient,
		cache:              cfg.Cache,
		expiration:         cfg.CacheTTL,
		compress:           cfg.Compress,
		compressionQuality: quality,
	}, nil
}

// IsVertexAI は、Vertex AI バックエンドを使用しているかを確認します。
func (c *GeminiImageCore) IsVertexAI() bool {
	return c.aiClient.IsVertexAI()
}

// EnsureUploaded は指定された fileURI の画像を Gemini File API にアップロードし、
// アップロード先の URI を返します。すでにアップロード済みならキャッシュの URI を返します。
//
// 引数の Reader を受け取る gemini.FileManager.UploadFile とは役割が異なります
// （こちらは「URL から取得してアップロードするところまで」を担います）。
func (c *GeminiImageCore) EnsureUploaded(ctx context.Context, fileURI string) (string, error) {
	if entry, ok := c.lookupCache(fileURI); ok {
		return entry.URI, nil
	}

	rc, err := c.fetchImageData(ctx, fileURI)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	br := bufio.NewReader(rc)
	mimeType, err := detectUploadSource(br)
	if err != nil {
		return "", err
	}

	uploaded, err := c.uploadByStrategy(ctx, br, mimeType, fileURI)
	if err != nil {
		return "", err
	}

	c.storeCache(fileURI, cachedFile{URI: uploaded.URI, Name: uploaded.Name})
	return uploaded.URI, nil
}

// DeleteFile は指定された URI を使用して Gemini File API からファイルを削除します。
// 削除に成功した場合は、同じソース URI での再利用を防ぐためキャッシュも無効化します。
func (c *GeminiImageCore) DeleteFile(ctx context.Context, fileURI string) error {
	entry, ok := c.lookupCache(fileURI)
	if !ok || entry.Name == "" {
		return fmt.Errorf("%w: %s", ErrFileNotInCache, fileURI)
	}
	if err := c.aiClient.DeleteFile(ctx, entry.Name); err != nil {
		return err
	}
	c.removeFromCache(fileURI)
	return nil
}

// PrepareImageAttachment は URL または cloud storage から画像を準備し、添付へ変換します。
func (c *GeminiImageCore) PrepareImageAttachment(ctx context.Context, rawURL string) (gemini.Attachment, error) {
	// キャッシュヒット時も非キャッシュ経路と同じ組み立てを通す。
	// ここで MIMEType を省くと、同じ画像でもキャッシュの有無で
	// API に送るペイロードが変わってしまう。
	if entry, ok := c.lookupCache(rawURL); ok {
		return fileAttachment(entry.URI, rawURL), nil
	}

	rc, err := c.fetchImageData(ctx, rawURL)
	if err != nil {
		return gemini.Attachment{}, fmt.Errorf("failed to fetch image data: %w", err)
	}
	defer rc.Close()

	rawData, err := io.ReadAll(rc)
	if err != nil {
		return gemini.Attachment{}, fmt.Errorf("failed to read image data: %w", err)
	}

	mimeType := imgutil.DetectMIMEType(rawData)
	if !imgutil.IsImageMIMEType(mimeType) {
		return gemini.Attachment{}, fmt.Errorf("%w: %s", ErrUnsupportedFileFormat, mimeType)
	}

	finalData := rawData
	if c.shouldCompress(mimeType) {
		compressed, err := imgutil.CompressToJPEG(bytes.NewReader(rawData), c.compressionQuality)
		if err != nil {
			return gemini.Attachment{}, fmt.Errorf("failed to compress image: %w", err)
		}
		finalData = compressed
		mimeType = "image/jpeg"
	}

	return gemini.Attachment{MIMEType: mimeType, Data: finalData}, nil
}

// ExecuteRequest は Gemini API を呼び出し、レスポンスをパースします。
func (c *GeminiImageCore) ExecuteRequest(ctx context.Context, model string, prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions) (*ports.ImageResponse, error) {
	resp, err := c.aiClient.GenerateWithAttachments(ctx, model, prompt, attachments, opts)
	if err != nil {
		return nil, err
	}

	return c.parseToResponse(resp, ports.DereferenceSeed(opts.Seed))
}

// parseToResponse は Gemini からのレスポンスから画像データを抽出します。
// FinishReason の検証（安全フィルターによるブロック等）は下層の go-gemini-client が行い、
// ブロック時は生成呼び出し自体がエラーを返すため、ここでは行いません。
func (c *GeminiImageCore) parseToResponse(resp *gemini.Response, seed int64) (*ports.ImageResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("%w: no response", ErrNoImageData)
	}

	// Response.Attachments は MIME type 込みで返るため、保存時の Content-Type を決めるために
	// 生の SDK レスポンスを辿る必要はありません。
	for _, attachment := range resp.Attachments {
		if len(attachment.Data) == 0 {
			continue
		}
		return &ports.ImageResponse{
			Data:     attachment.Data,
			MimeType: attachment.MIMEType,
			UsedSeed: seed,
		}, nil
	}

	return nil, fmt.Errorf("%w: response contains no inline image", ErrNoImageData)
}
