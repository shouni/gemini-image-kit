package generator

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	httpClient httpkit.StreamDownloader
	cache      ImageCacher
	expiration time.Duration
}

// NewGeminiImageCore は依存関係を注入して GeminiImageCore を初期化します。
func NewGeminiImageCore(aiClient gemini.GenerativeModel, reader remoteio.InputReader, httpClient httpkit.StreamDownloader, cache ImageCacher, cacheTTL time.Duration) (*GeminiImageCore, error) {
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

	// 1. ストリームとして画像を取得
	rc, err := c.fetchImageData(ctx, fileURI)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	rawData, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("画像データの読み込みに失敗しました: %w", err)
	}

	// 2. 圧縮処理のパイプライン
	var finalData []byte

	if UseImageCompression {
		compressed, err := imgutil.CompressToJPEG(bytes.NewReader(rawData), ImageCompressionQuality)
		if err == nil {
			finalData = compressed
		} else {
			finalData = rawData
		}
	} else {
		finalData = rawData
	}

	mimeType := http.DetectContentType(finalData)
	displayName := filepath.Base(fileURI)

	// 3. アップロード処理へ渡す
	uri, fileName, err := c.aiClient.UploadFile(ctx, bytes.NewReader(finalData), mimeType, displayName)
	if err != nil {
		return "", err
	}

	// 4. URI と Name をキャッシュ
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

// IsVertexAI は、Vertex AI バックエンドを使用しているかを確認します。
func (c *GeminiImageCore) IsVertexAI() bool {
	return c.aiClient.IsVertexAI()
}
