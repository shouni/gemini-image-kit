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
	"google.golang.org/genai"

	"github.com/shouni/gemini-image-kit/imgutil"
	"github.com/shouni/gemini-image-kit/ports"
)

const (
	// ImageCompressionQuality は、画像圧縮時に使用するJPEG品質値です。
	ImageCompressionQuality = 75
	cacheKeyFileAPI         = "fileapi:"
)

// GeminiImageCore は AssetManager と ImageExecutor の両方の責務を担う基盤クラスです。
type GeminiImageCore struct {
	aiClient   gemini.GenerativeModel
	reader     ports.ContentReader
	httpClient ports.Downloader
	cache      ports.ImageCacher
	expiration time.Duration
	compress   bool
}

// NewGeminiImageCore は依存関係を注入して GeminiImageCore を初期化します。
func NewGeminiImageCore(
	aiClient gemini.GenerativeModel,
	reader ports.ContentReader,
	httpClient ports.Downloader,
	cache ports.ImageCacher,
	cacheTTL time.Duration,
	compress bool,
) (*GeminiImageCore, error) {
	if aiClient == nil {
		return nil, ErrAIClientRequired
	}
	if reader == nil {
		return nil, ErrReaderRequired
	}
	if httpClient == nil {
		return nil, ErrHTTPClientRequired
	}
	// cache は任意ではありません。DeleteFile が File API 上のファイル名を
	// キャッシュから引くため、nil だと削除が一切できなくなります。
	if cache == nil {
		return nil, ErrCacheRequired
	}

	return &GeminiImageCore{
		aiClient:   aiClient,
		reader:     reader,
		httpClient: httpClient,
		cache:      cache,
		expiration: cacheTTL,
		compress:   compress,
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

// PrepareImagePart は URL または cloud storageから画像を準備し、genai.Part に変換します。
func (c *GeminiImageCore) PrepareImagePart(ctx context.Context, rawURL string) (*genai.Part, error) {
	// キャッシュヒット時も非キャッシュ経路と同じ組み立てを通す。
	// ここで MIMEType を省くと、同じ画像でもキャッシュの有無で
	// API に送るペイロードが変わってしまう。
	if entry, ok := c.lookupCache(rawURL); ok {
		return buildFileDataPart(entry.URI, rawURL), nil
	}

	rc, err := c.fetchImageData(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image data: %w", err)
	}
	defer rc.Close()

	rawData, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	mimeType := imgutil.DetectMIMEType(rawData)
	if !imgutil.IsImageMIMEType(mimeType) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFileFormat, mimeType)
	}

	finalData := rawData
	if c.shouldCompress(mimeType) {
		compressed, err := imgutil.CompressToJPEG(bytes.NewReader(rawData), ImageCompressionQuality)
		if err != nil {
			return nil, fmt.Errorf("failed to compress image: %w", err)
		}
		finalData = compressed
		mimeType = "image/jpeg"
	}

	return &genai.Part{InlineData: &genai.Blob{MIMEType: mimeType, Data: finalData}}, nil
}

// ExecuteRequest は Gemini API を呼び出し、レスポンスをパースします。
func (c *GeminiImageCore) ExecuteRequest(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*ports.ImageResponse, error) {
	resp, err := c.aiClient.GenerateWithParts(ctx, model, parts, opts)
	if err != nil {
		return nil, err
	}

	return c.parseToResponse(resp, ports.DereferenceSeed(opts.Seed))
}

// parseToResponse は Gemini からのレスポンスから画像データを抽出します。
// FinishReason の検証（安全フィルターによるブロック等）は下層の go-gemini-client が行い、
// ブロック時は GenerateWithParts 自体がエラーを返すため、ここでは行いません。
func (c *GeminiImageCore) parseToResponse(resp *gemini.Response, seed int64) (*ports.ImageResponse, error) {
	if resp == nil || resp.RawResponse == nil || len(resp.RawResponse.Candidates) == 0 {
		return nil, ErrEmptyResponse
	}

	candidate := resp.RawResponse.Candidates[0]
	if candidate.Content == nil {
		return nil, fmt.Errorf("%w: no content in candidate", ErrNoImageData)
	}

	for _, part := range candidate.Content.Parts {
		if part.InlineData != nil {
			return &ports.ImageResponse{
				Data:     part.InlineData.Data,
				MimeType: part.InlineData.MIMEType,
				UsedSeed: seed,
			}, nil
		}
	}

	return nil, fmt.Errorf("%w: parts contain no inline image", ErrNoImageData)
}
