package generator

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/shouni/gemini-image-kit/pkg/imgutil"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/shouni/go-remote-io/pkg/remoteio"
)

// GeminiImageCore は AssetManager と ImageExecutor の両方の責務を担う基盤クラスです。
type GeminiImageCore struct {
	aiClient   gemini.GenerativeModel
	reader     remoteio.InputReader
	httpClient httpkit.ClientInterface
	cache      ImageCacher
	expiration time.Duration
}

// NewGeminiImageCore は依存関係を注入して GeminiImageCore を初期化します。
func NewGeminiImageCore(aiClient gemini.GenerativeModel, reader remoteio.InputReader, httpClient httpkit.ClientInterface, cache ImageCacher, cacheTTL time.Duration) (*GeminiImageCore, error) {
	// どの依存関係が不足しているか具体的に示すように修正
	if aiClient == nil {
		return nil, fmt.Errorf("aiClient is required")
	}
	if reader == nil {
		return nil, fmt.Errorf("reader is required")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("httpClient is required")
	}
	// cache は nil を許容（キャッシュなし動作）

	return &GeminiImageCore{
		aiClient:   aiClient,
		reader:     reader,
		httpClient: httpClient,
		cache:      cache,
		expiration: cacheTTL,
	}, nil
}

// UploadFile は画像を Gemini File API にアップロードし、URI を返します。
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

	// File API へのアップロード
	uri, fileName, err := c.aiClient.UploadFile(ctx, finalData, mimeType, displayName)
	if err != nil {
		return "", err
	}

	// URI（参照用）と Name（削除用）の両方をキャッシュ
	if c.cache != nil {
		c.cache.Set(cacheKeyURI, uri, c.expiration)
		c.cache.Set(cacheKeyFileAPIName+fileURI, fileName, c.expiration)
	}

	return uri, nil
}

// DeleteFile はキャッシュされたファイル名を使用して Gemini File API からファイルを削除します。
func (c *GeminiImageCore) DeleteFile(ctx context.Context, fileURI string) error {
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyFileAPIName + fileURI); ok {
			if name, ok := val.(string); ok {
				// 正しいファイル名 (files/xxxx) で削除を実行
				return c.aiClient.DeleteFile(ctx, name)
			}
		}
	}

	// キャッシュミスした場合、URL 形式の fileURI では Delete API を叩けないためエラーを返す
	return fmt.Errorf("cannot determine file name for deletion, file not found in cache: %s", fileURI)
}
