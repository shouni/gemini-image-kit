package generator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/shouni/gemini-image-kit/pkg/imgutil"
	"github.com/shouni/go-remote-io/pkg/remoteio"
	"google.golang.org/genai"
)

// fetchImageData は、指定されたURLまたはCloud Storageから画像データ読み込み用の Reader を返します。
// 呼び出し側は、読み込み終了後に必ず Close() を呼び出す必要があります。
func (c *GeminiImageCore) fetchImageData(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	// 1. Cloud Storage の場合
	if remoteio.IsRemoteURI(rawURL) {
		return c.reader.Open(ctx, rawURL)
	}

	// 2. HTTP/HTTPS の場合
	return c.httpClient.GetStream(ctx, rawURL)
}

// toPart は、与えられたデータが有効な画像MIMEタイプを持つ場合に genai.Part オブジェクトへ変換します。
func (c *GeminiImageCore) toPart(data []byte) *genai.Part {
	mimeType := http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "image/") {
		return nil
	}
	return &genai.Part{InlineData: &genai.Blob{MIMEType: mimeType, Data: data}}
}

// uploadCompressed は画像を圧縮してからアップロードする処理です。
func (c *GeminiImageCore) uploadCompressed(ctx context.Context, rc io.Reader, fileURI string) (string, string, error) {
	compressed, err := imgutil.CompressToJPEG(rc, ImageCompressionQuality)
	if err != nil {
		return "", "", fmt.Errorf("画像の圧縮に失敗しました: %w", err)
	}

	mimeType := http.DetectContentType(compressed)
	return c.aiClient.UploadFile(ctx, bytes.NewReader(compressed), mimeType, filepath.Base(fileURI))
}

// uploadStream はストリームをそのままアップロードする処理です。
func (c *GeminiImageCore) uploadStream(ctx context.Context, rc io.Reader, fileURI string) (string, string, error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(rc, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", "", fmt.Errorf("画像ヘッダの読み込みに失敗しました: %w", err)
	}
	if n == 0 {
		return "", "", fmt.Errorf("画像データが空です")
	}

	mimeType := http.DetectContentType(head[:n])
	stream := io.MultiReader(bytes.NewReader(head[:n]), rc)

	return c.aiClient.UploadFile(ctx, stream, mimeType, filepath.Base(fileURI))
}

func (c *GeminiImageCore) getFromCache(fileURI string) (string, bool) {
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyFileAPIURI + fileURI); ok {
			if uri, ok := val.(string); ok {
				return uri, true
			}
		}
	}
	return "", false
}

func (c *GeminiImageCore) saveToCache(fileURI, uri, fileName string) {
	if c.cache != nil {
		c.cache.Set(cacheKeyFileAPIURI+fileURI, uri, c.expiration)
		c.cache.Set(cacheKeyFileAPIName+fileURI, fileName, c.expiration)
	}
}
