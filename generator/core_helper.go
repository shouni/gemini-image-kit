package generator

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/shouni/gemini-image-kit/imgutil"
)

// fetchImageData は、指定されたURLまたはCloud Storageから画像データ読み込み用の Reader を返します。
// 呼び出し側は、読み込み終了後に必ず Close() を呼び出す必要があります。
func (c *GeminiImageCore) fetchImageData(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	// 1. Cloud Storage の場合
	if IsGCSURI(rawURL) {
		return c.reader.Open(ctx, rawURL)
	}

	// 2. HTTP/HTTPS の場合
	return c.httpClient.GetStream(ctx, rawURL)
}

// uploadStream はストリームをそのままアップロードします。
func (c *GeminiImageCore) uploadStream(ctx context.Context, r io.Reader, mimeType, fileURI string) (string, string, error) {
	return c.aiClient.UploadFile(ctx, r, mimeType, filepath.Base(fileURI))
}

// uploadCompressed は画像をJPEGに圧縮してからアップロードします。
func (c *GeminiImageCore) uploadCompressed(ctx context.Context, r io.Reader, fileURI string) (string, string, error) {
	compressed, err := imgutil.CompressToJPEG(r, ImageCompressionQuality)
	if err != nil {
		return "", "", fmt.Errorf("failed to compress image for upload: %w", err)
	}
	return c.aiClient.UploadFile(ctx, bytes.NewReader(compressed), "image/jpeg", filepath.Base(fileURI))
}

// uploadByStrategy は、画像の圧縮設定に基づいてアップロードを実行します。
func (c *GeminiImageCore) uploadByStrategy(ctx context.Context, br *bufio.Reader, mimeType, fileURI string) (string, string, error) {
	if c.compress && imgutil.IsCompressibleMimeType(mimeType) {
		return c.uploadCompressed(ctx, br, fileURI)
	}
	return c.uploadStream(ctx, br, mimeType, fileURI)
}

// cacheGetString は、キャッシュから文字列を取得します。存在しない場合は空文字列と false を返します。
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

// getFromCache は、キャッシュからファイルのAPI URIを取得します。存在しない場合は空文字列と false を返します。
func (c *GeminiImageCore) getFromCache(fileURI string) (string, bool) {
	return c.cacheGetString(cacheKeyFileAPIURI + fileURI)
}

// saveToCache は、キャッシュにファイルのAPI URIとファイル名を保存します。
func (c *GeminiImageCore) saveToCache(fileURI, uri, fileName string) {
	if c.cache != nil {
		c.cache.Set(cacheKeyFileAPIURI+fileURI, uri, c.expiration)
		c.cache.Set(cacheKeyFileAPIName+fileURI, fileName, c.expiration)
	}
}

// removeFromCache は、指定されたソース URI に紐づくキャッシュエントリを削除します。
func (c *GeminiImageCore) removeFromCache(fileURI string) {
	if c.cache != nil {
		c.cache.Delete(cacheKeyFileAPIURI + fileURI)
		c.cache.Delete(cacheKeyFileAPIName + fileURI)
	}
}

// detectUploadSource は、バッファ付きリーダーに有効な画像データが含まれていることを検証し、MIMETypeを返します。
func detectUploadSource(br *bufio.Reader) (string, error) {
	head, err := br.Peek(512)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("画像ヘッダの読み込みに失敗しました: %w", err)
	}
	if len(head) == 0 {
		return "", fmt.Errorf("画像データが空です")
	}

	detectedMime := imgutil.DetectMIMEType(head)
	if !imgutil.IsImageMIMEType(detectedMime) {
		return "", fmt.Errorf("%w (コンテンツ判定: %s)", ErrUnsupportedFileFormat, detectedMime)
	}
	return detectedMime, nil
}

// IsGCSURI は、指定されたURIがGCS（Google Cloud Storage）のストレージURIであるかどうかを判定します。
func IsGCSURI(uri string) bool {
	const prefixGCS = "gs://"
	return strings.HasPrefix(uri, prefixGCS)
}
