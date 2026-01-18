package generator

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/gemini-image-kit/pkg/imgutil"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"github.com/shouni/go-remote-io/pkg/remoteio"
	"google.golang.org/genai"
)

// GeminiImageCore は AssetManager と ImageGeneratorCore の両方を実装します。
type GeminiImageCore struct {
	aiClient   gemini.GenerativeModel
	reader     remoteio.InputReader
	httpClient HTTPClient
	cache      ImageCacher
	expiration time.Duration
}

func NewGeminiImageCore(aiClient gemini.GenerativeModel, reader remoteio.InputReader, client HTTPClient, cache ImageCacher, cacheTTL time.Duration) (*GeminiImageCore, error) {
	if aiClient == nil || reader == nil || client == nil {
		return nil, fmt.Errorf("required dependencies are missing")
	}
	return &GeminiImageCore{
		aiClient:   aiClient,
		reader:     reader,
		httpClient: client,
		cache:      cache,
		expiration: cacheTTL,
	}, nil
}

func (c *GeminiImageCore) UploadFile(ctx context.Context, fileURI string) (string, error) {
	cacheKeyURI := cacheKeyFileAPIURI + fileURI
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyURI); ok {
			if uri, ok := val.(string); ok {
				return uri, nil
			}
		}
	}

	data, err := c.fetchImageData(ctx, fileURI)
	if err != nil {
		return "", err
	}

	finalData := data
	if UseImageCompression {
		if compressed, err := imgutil.CompressToJPEG(data, ImageCompressionQuality); err == nil {
			finalData = compressed
		}
	}

	mimeType := http.DetectContentType(finalData)
	displayName := filepath.Base(fileURI)
	uri, fileName, err := c.aiClient.UploadFile(ctx, finalData, mimeType, displayName)
	if err != nil {
		return "", err
	}

	if c.cache != nil {
		c.cache.Set(cacheKeyURI, uri, c.expiration)
		c.cache.Set(cacheKeyFileAPIName+fileURI, fileName, c.expiration)
	}

	return uri, nil
}

func (c *GeminiImageCore) DeleteFile(ctx context.Context, fileURI string) error {
	targetName := fileURI
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyFileAPIName + fileURI); ok {
			if name, ok := val.(string); ok {
				targetName = name
			}
		}
	}
	return c.aiClient.DeleteFile(ctx, targetName)
}

func (c *GeminiImageCore) ExecuteRequest(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*domain.ImageResponse, error) {
	gOpts := gemini.GenerateOptions{
		AspectRatio:  opts.AspectRatio,
		SystemPrompt: opts.SystemPrompt,
		Seed:         opts.Seed,
	}

	resp, err := c.aiClient.GenerateWithParts(ctx, model, parts, gOpts)
	if err != nil {
		return nil, err
	}

	out, err := c.parseToResponse(resp, c.dereferenceSeed(opts.Seed))
	if err != nil {
		return nil, err
	}

	return &domain.ImageResponse{
		Data:     out.Data,
		MimeType: out.MimeType,
		UsedSeed: out.UsedSeed,
	}, nil
}

func (c *GeminiImageCore) PrepareImagePart(ctx context.Context, rawURL string) *genai.Part {
	// File API キャッシュチェック
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyFileAPIURI + rawURL); ok {
			if uri, ok := val.(string); ok {
				return &genai.Part{FileData: &genai.FileData{FileURI: uri}}
			}
		}
	}

	// 取得と圧縮
	data, err := c.fetchImageData(ctx, rawURL)
	if err != nil {
		return nil
	}
	finalData := data
	if UseImageCompression {
		if compressed, err := imgutil.CompressToJPEG(data, ImageCompressionQuality); err == nil {
			finalData = compressed
		}
	}

	return c.toPart(finalData)
}
