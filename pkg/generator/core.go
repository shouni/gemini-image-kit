package generator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/gemini-image-kit/pkg/imgutil"
	"github.com/shouni/go-gemini-client/pkg/gemini"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-remote-io/pkg/remoteio"
	"google.golang.org/genai"
)

// GeminiImageCore は AssetManager と ImageExecutor の両方の責務を担う基盤クラスです。
type GeminiImageCore struct {
	aiClient            gemini.GenerativeModel
	reader              remoteio.InputReader
	httpClient          httpkit.StreamDownloader
	cache               ImageCacher
	expiration          time.Duration
	useImageCompression bool
}

// NewGeminiImageCore は依存関係を注入して GeminiImageCore を初期化します。
func NewGeminiImageCore(aiClient gemini.GenerativeModel, reader remoteio.InputReader, httpClient httpkit.StreamDownloader, cache ImageCacher, cacheTTL time.Duration, useImageCompression bool) (*GeminiImageCore, error) {
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
		aiClient:            aiClient,
		reader:              reader,
		httpClient:          httpClient,
		cache:               cache,
		expiration:          cacheTTL,
		useImageCompression: useImageCompression,
	}, nil
}

// UploadFile は画像を Gemini File API にアップロードし、URI を返します。
func (c *GeminiImageCore) UploadFile(ctx context.Context, fileURI string) (string, error) {
	if uri, ok := c.getFromCache(fileURI); ok {
		return uri, nil
	}

	rc, err := c.fetchImageData(ctx, fileURI)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	var uri, fileName string
	if c.useImageCompression {
		uri, fileName, err = c.uploadCompressed(ctx, rc, fileURI)
	} else {
		uri, fileName, err = c.uploadStream(ctx, rc, fileURI)
	}

	if err != nil {
		return "", err
	}

	c.saveToCache(fileURI, uri, fileName)
	return uri, nil
}

// DeleteFile はキャッシュされたファイル名を使用して Gemini File API からファイルを削除します。
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

// IsVertexAI は、Vertex AI バックエンドを使用しているかを確認します。
func (c *GeminiImageCore) IsVertexAI() bool {
	return c.aiClient.IsVertexAI()
}

// ExecuteRequest は Gemini API を呼び出し、レスポンスをパースします。
func (c *GeminiImageCore) ExecuteRequest(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*domain.ImageResponse, error) {
	resp, err := c.aiClient.GenerateWithParts(ctx, model, parts, opts)
	if err != nil {
		return nil, err
	}

	out, err := c.ParseToResponse(resp, domain.DereferenceSeed(opts.Seed))
	if err != nil {
		return nil, err
	}

	return &domain.ImageResponse{
		Data:     out.Data,
		MimeType: out.MimeType,
		UsedSeed: out.UsedSeed,
	}, nil
}

// PrepareImagePart は URL または cloud storageから画像を準備し、genai.Part に変換します。
func (c *GeminiImageCore) PrepareImagePart(ctx context.Context, rawURL string) *genai.Part {
	// 1. File API キャッシュチェック
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyFileAPIURI + rawURL); ok {
			if uri, ok := val.(string); ok {
				return &genai.Part{FileData: &genai.FileData{FileURI: uri}}
			}
		}
	}

	// 2. 画像の取得（ストリーム処理）
	rc, err := c.fetchImageData(ctx, rawURL)
	if err != nil {
		return nil
	}
	defer rc.Close()

	rawData, err := io.ReadAll(rc)
	if err != nil {
		return nil
	}

	// 3. 画像圧縮処理
	finalData := rawData
	if c.useImageCompression {
		if compressed, err := imgutil.CompressToJPEG(bytes.NewReader(rawData), ImageCompressionQuality); err == nil {
			finalData = compressed
		}
	}

	return c.toPart(finalData)
}
