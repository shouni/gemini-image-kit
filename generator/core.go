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
	cacheKeyFileAPIURI      = "fileapi_uri:"
	cacheKeyFileAPIName     = "fileapi_name:"
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
		return nil, fmt.Errorf("aiClient is required")
	}
	if reader == nil {
		return nil, fmt.Errorf("reader is required")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("httpClient is required")
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

// UploadFile は指定された fileURI の画像を Gemini File API にアップロードし、アップロード先の URI を返します。
func (c *GeminiImageCore) UploadFile(ctx context.Context, fileURI string) (string, error) {
	if uri, ok := c.getFromCache(fileURI); ok {
		return uri, nil
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

	uri, fileName, err := c.uploadByStrategy(ctx, br, mimeType, fileURI)
	if err != nil {
		return "", err
	}

	c.saveToCache(fileURI, uri, fileName)
	return uri, nil
}

// DeleteFile は指定された URI を使用して Gemini File API からファイルを削除します。
// 削除に成功した場合は、同じソース URI での再利用を防ぐためキャッシュも無効化します。
func (c *GeminiImageCore) DeleteFile(ctx context.Context, fileURI string) error {
	name, ok := c.cacheGetString(cacheKeyFileAPIName + fileURI)
	if !ok {
		return fmt.Errorf("%w: %s", ErrFileNotInCache, fileURI)
	}
	if err := c.aiClient.DeleteFile(ctx, name); err != nil {
		return err
	}
	c.removeFromCache(fileURI)
	return nil
}

// PrepareImagePart は URL または cloud storageから画像を準備し、genai.Part に変換します。
func (c *GeminiImageCore) PrepareImagePart(ctx context.Context, rawURL string) (*genai.Part, error) {
	if uri, ok := c.cacheGetString(cacheKeyFileAPIURI + rawURL); ok {
		return &genai.Part{FileData: &genai.FileData{FileURI: uri}}, nil
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
	if c.compress && imgutil.IsCompressibleMimeType(mimeType) {
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

	return c.ParseToResponse(resp, ports.DereferenceSeed(opts.Seed))
}

// ParseToResponse は Gemini からのレスポンスから画像データを抽出します。
// FinishReason の検証（安全フィルターによるブロック等）は下層の go-gemini-client が行い、
// ブロック時は GenerateWithParts 自体がエラーを返すため、ここでは行いません。
func (c *GeminiImageCore) ParseToResponse(resp *gemini.Response, seed int64) (*ports.ImageResponse, error) {
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
