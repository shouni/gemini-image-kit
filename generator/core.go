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
func (c *GeminiImageCore) DeleteFile(ctx context.Context, fileURI string) error {
	if name, ok := c.cacheGetString(cacheKeyFileAPIName + fileURI); ok {
		return c.aiClient.DeleteFile(ctx, name)
	}
	return fmt.Errorf("cannot determine file name for deletion, file not found in cache: %s", fileURI)
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

	finalData := rawData
	mimeType := imgutil.DetectMIMEType(rawData)
	if !imgutil.IsImageMIMEType(mimeType) {
		return nil, fmt.Errorf("unsupported file format: %s", mimeType)
	}
	if c.compress && imgutil.IsCompressibleMimeType(mimeType) {
		compressed, err := imgutil.CompressToJPEG(bytes.NewReader(rawData), ImageCompressionQuality)
		if err != nil {
			return nil, fmt.Errorf("failed to compress image: %w", err)
		}
		finalData = compressed
	}

	part := c.toPart(finalData)
	if part == nil {
		return nil, fmt.Errorf("unsupported file format: %s", imgutil.DetectMIMEType(finalData))
	}
	return part, nil
}

// PrepareReferenceImage は URL または cloud storage から編集 API 用の参照画像を準備します。
func (c *GeminiImageCore) PrepareReferenceImage(ctx context.Context, rawURL string) (*genai.Image, error) {
	if c.IsVertexAI() && IsGCSURI(rawURL) {
		return &genai.Image{
			GCSURI:   rawURL,
			MIMEType: imgutil.GuessMIMEType(rawURL),
		}, nil
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
		return nil, fmt.Errorf("unsupported file format: %s", mimeType)
	}

	return &genai.Image{
		ImageBytes: rawData,
		MIMEType:   mimeType,
	}, nil
}

// ExecuteRequest は Gemini API を呼び出し、レスポンスをパースします。
func (c *GeminiImageCore) ExecuteRequest(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*ports.ImageResponse, error) {
	resp, err := c.aiClient.GenerateWithParts(ctx, model, parts, opts)
	if err != nil {
		return nil, err
	}

	return c.ParseToResponse(resp, ports.DereferenceSeed(opts.Seed))
}

// ExecuteEditRequest は Gemini の画像編集 API を呼び出し、レスポンスをパースします。
func (c *GeminiImageCore) ExecuteEditRequest(ctx context.Context, model string, prompt string, referenceImages []genai.ReferenceImage, config *genai.EditImageConfig, seed int64) (*ports.ImageResponse, error) {
	resp, err := c.aiClient.EditImage(ctx, model, prompt, referenceImages, config)
	if err != nil {
		return nil, err
	}

	return c.ParseEditToResponse(resp, seed)
}

// ParseToResponse は Gemini からのレスポンスを検証し、画像データを抽出します。
func (c *GeminiImageCore) ParseToResponse(resp *gemini.Response, seed int64) (*ports.ImageResponse, error) {
	if resp == nil || resp.RawResponse == nil || len(resp.RawResponse.Candidates) == 0 {
		return nil, fmt.Errorf("invalid or empty response from Gemini")
	}

	candidate := resp.RawResponse.Candidates[0]

	if candidate.FinishReason != genai.FinishReasonStop && candidate.FinishReason != genai.FinishReasonUnspecified {
		return nil, fmt.Errorf("generation failed with FinishReason: %s", candidate.FinishReason)
	}

	if candidate.Content == nil {
		return nil, fmt.Errorf("no content found in candidate")
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

	return nil, fmt.Errorf("no image data found in response parts")
}

// ParseEditToResponse は Gemini の画像編集レスポンスを検証し、画像データを抽出します。
func (c *GeminiImageCore) ParseEditToResponse(resp *genai.EditImageResponse, seed int64) (*ports.ImageResponse, error) {
	if resp == nil || len(resp.GeneratedImages) == 0 {
		return nil, fmt.Errorf("invalid or empty edit response from Gemini")
	}

	for _, generated := range resp.GeneratedImages {
		if generated != nil && generated.Image != nil {
			return &ports.ImageResponse{
				Data:     generated.Image.ImageBytes,
				MimeType: generated.Image.MIMEType,
				UsedSeed: seed,
			}, nil
		}
	}

	return nil, fmt.Errorf("no image data found in edit response")
}
