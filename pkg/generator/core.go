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
// ストリーム最適化により、圧縮不要時はメモリ消費を最小限に抑えてアップロードします。
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
	// 圧縮・アップロードが完了するまでクローズを遅延
	defer rc.Close()

	var uri, fileName string

	// 2. 圧縮パイプラインまたはストリームアップロード
	if c.useImageCompression == true {
		// 圧縮時はメモリバッファリングが必要
		rawData, err := io.ReadAll(rc)
		if err != nil {
			return "", fmt.Errorf("画像データの読み込みに失敗しました: %w", err)
		}

		compressed, err := imgutil.CompressToJPEG(bytes.NewReader(rawData), ImageCompressionQuality)
		finalData := rawData
		if err == nil {
			finalData = compressed
		}

		mimeType := http.DetectContentType(finalData)
		uri, fileName, err = c.aiClient.UploadFile(ctx, bytes.NewReader(finalData), mimeType, filepath.Base(fileURI))
	} else {
		// 圧縮不要時は MultiReader でストリームを直接アップロード
		head := make([]byte, 512)
		n, err := io.ReadFull(rc, head)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return "", fmt.Errorf("画像ヘッダの読み込みに失敗しました: %w", err)
		}

		mimeType := http.DetectContentType(head[:n])
		stream := io.MultiReader(bytes.NewReader(head[:n]), rc)

		uri, fileName, err = c.aiClient.UploadFile(ctx, stream, mimeType, filepath.Base(fileURI))
	}

	if err != nil {
		return "", err
	}

	// 3. URI と Name をキャッシュ
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
