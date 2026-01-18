package generator

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/shouni/gemini-image-kit/pkg/imgutil"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"github.com/shouni/go-remote-io/pkg/remoteio"
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
