package generator

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/shouni/go-remote-io/remoteio"
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

// uploadStream はストリームをそのままアップロードします。
func (c *GeminiImageCore) uploadStream(ctx context.Context, r io.Reader, mimeType, fileURI string) (string, string, error) {
	return c.aiClient.UploadFile(ctx, r, mimeType, filepath.Base(fileURI))
}

// uploadCompressed は画像を圧縮してからアップロードします。
func (c *GeminiImageCore) uploadCompressed(ctx context.Context, r io.Reader, mimeType, fileURI string) (string, string, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode image for compression: %w", err)
	}

	pr, pw := io.Pipe()
	go func() {
		err := jpeg.Encode(pw, img, &jpeg.Options{Quality: ImageCompressionQuality})
		pw.CloseWithError(err)
	}()

	defer pr.Close()
	return c.aiClient.UploadFile(ctx, pr, mimeType, filepath.Base(fileURI))
}

func (c *GeminiImageCore) cacheGetString(key string) (string, bool) {
	if c.cache == nil {
		return "", false
	}
	val, ok := c.cache.Get(key)
	if !ok {
		return "", false
	}
	strVal, ok := val.(string)
	if !ok {
		return "", false
	}
	return strVal, true
}

func (c *GeminiImageCore) getFromCache(fileURI string) (string, bool) {
	return c.cacheGetString(cacheKeyFileAPIURI + fileURI)
}

func (c *GeminiImageCore) saveToCache(fileURI, uri, fileName string) {
	if c.cache != nil {
		c.cache.Set(cacheKeyFileAPIURI+fileURI, uri, c.expiration)
		c.cache.Set(cacheKeyFileAPIName+fileURI, fileName, c.expiration)
	}
}
