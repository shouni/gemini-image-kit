package generator

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-remote-io/pkg/remoteio"
	"google.golang.org/genai"

	"github.com/shouni/gemini-image-kit/pkg/imgutil"
	"github.com/shouni/gemini-image-kit/pkg/ports"
)

const (
	ImageCompressionQuality = 75
	cacheKeyFileAPIURI      = "fileapi_uri:"
	cacheKeyFileAPIName     = "fileapi_name:"
)

// GeminiImageCore は AssetManager と ImageExecutor の両方の責務を担う基盤クラスです。
type GeminiImageCore struct {
	aiClient   gemini.GenerativeModel
	reader     remoteio.InputReader
	httpClient httpkit.StreamDownloader
	cache      ports.ImageCacher
	expiration time.Duration
	compress   bool
}

// NewGeminiImageCore は依存関係を注入して GeminiImageCore を初期化します。
func NewGeminiImageCore(aiClient gemini.GenerativeModel, reader remoteio.InputReader, httpClient httpkit.StreamDownloader, cache ports.ImageCacher, cacheTTL time.Duration, compress bool) (*GeminiImageCore, error) {
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

	// 1. ファイルの中身を少しだけ先読み（バリデーション用）
	head, err := br.Peek(512)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("画像ヘッダの読み込みに失敗しました: %w", err)
	}
	if len(head) == 0 {
		return "", fmt.Errorf("画像データが空です")
	}

	// 2. コンテンツベースで「画像であること」を厳格にチェック
	detectedMime := http.DetectContentType(head)
	if !strings.HasPrefix(detectedMime, "image/") {
		return "", fmt.Errorf("サポートされていないファイル形式です (コンテンツ判定: %s)", detectedMime)
	}

	mimeType := imgutil.GuessMIMEType(fileURI)
	var uri, fileName string
	if c.compress && imgutil.IsCompressibleMimeType(mimeType) {
		uri, fileName, err = c.uploadCompressed(ctx, br, mimeType, fileURI)
	} else {
		uri, fileName, err = c.uploadStream(ctx, br, mimeType, fileURI)
	}

	if err != nil {
		return "", err
	}

	c.saveToCache(fileURI, uri, fileName)
	return uri, nil
}

// DeleteFile は指定された URI を使用して Gemini File API からファイルを削除します。
func (c *GeminiImageCore) DeleteFile(ctx context.Context, fileURI string) error {
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyFileAPIName + fileURI); ok {
			if name, ok := val.(string); ok {
				return c.aiClient.DeleteFile(ctx, name)
			}
		}
	}
	return fmt.Errorf("cannot determine file name for deletion, file not found in cache: %s", fileURI)
}

// PrepareImagePart は URL または cloud storageから画像を準備し、genai.Part に変換します。
func (c *GeminiImageCore) PrepareImagePart(ctx context.Context, rawURL string) *genai.Part {
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyFileAPIURI + rawURL); ok {
			if uri, ok := val.(string); ok {
				return &genai.Part{FileData: &genai.FileData{FileURI: uri}}
			}
		}
	}

	rc, err := c.fetchImageData(ctx, rawURL)
	if err != nil {
		return nil
	}
	defer rc.Close()

	rawData, err := io.ReadAll(rc)
	if err != nil {
		return nil
	}

	finalData := rawData
	if c.compress {
		compressed, err := imgutil.CompressToJPEG(bytes.NewReader(rawData), ImageCompressionQuality)
		if err == nil {
			finalData = compressed
		}
	}

	return c.toPart(finalData)
}

// ExecuteRequest は Gemini API を呼び出し、レスポンスをパースします。
func (c *GeminiImageCore) ExecuteRequest(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*ports.ImageResponse, error) {
	resp, err := c.aiClient.GenerateWithParts(ctx, model, parts, opts)
	if err != nil {
		return nil, err
	}

	return c.ParseToResponse(resp, ports.DereferenceSeed(opts.Seed))
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
