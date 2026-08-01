package generator

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/shouni/gemini-image-kit/imgutil"
	"github.com/shouni/go-gemini-client/gemini"
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
func (c *GeminiImageCore) uploadStream(ctx context.Context, r io.Reader, mimeType, fileURI string) (gemini.UploadedFile, error) {
	return c.aiClient.UploadFile(ctx, r, mimeType, uploadDisplayName(fileURI))
}

// uploadCompressed は画像をJPEGに圧縮してからアップロードします。
func (c *GeminiImageCore) uploadCompressed(ctx context.Context, r io.Reader, fileURI string) (gemini.UploadedFile, error) {
	compressed, err := imgutil.CompressToJPEG(r, c.compressionQuality)
	if err != nil {
		return gemini.UploadedFile{}, fmt.Errorf("failed to compress image for upload: %w", err)
	}
	return c.aiClient.UploadFile(ctx, bytes.NewReader(compressed), "image/jpeg", uploadDisplayName(fileURI))
}

// uploadByStrategy は、画像の圧縮設定に基づいてアップロードを実行します。
func (c *GeminiImageCore) uploadByStrategy(ctx context.Context, br *bufio.Reader, mimeType, fileURI string) (gemini.UploadedFile, error) {
	if c.shouldCompress(mimeType) {
		return c.uploadCompressed(ctx, br, fileURI)
	}
	return c.uploadStream(ctx, br, mimeType, fileURI)
}

// uploadDisplayName は、ソース URI から File API に付ける表示名を作ります。
//
// URI をそのまま filepath.Base に掛けると、署名付き URL では
// "photo.png?X-Goog-Signature=..." のようにクエリ文字列ごと表示名になってしまうため、
// パス部分だけを見ます。解析できない文字列はそのまま Base に掛けます。
func uploadDisplayName(rawURI string) string {
	if u, err := url.Parse(rawURI); err == nil && u.Path != "" {
		return path.Base(u.Path)
	}
	return filepath.Base(rawURI)
}

// shouldCompress は、指定された MIMEType の画像を圧縮すべきかを判定します。
func (c *GeminiImageCore) shouldCompress(mimeType string) bool {
	return c.compress && imgutil.IsCompressibleMimeType(mimeType)
}

// cachedFile は File API 上のファイル参照です。
//
// URI と Name を1エントリにまとめて保存します。別々のキーに分けると、
// 片方だけが失効したときに「生成には使えるが削除できない」中途半端な状態が
// 生まれるためです（DeleteFile は Name に依存します）。
type cachedFile struct {
	URI  string
	Name string
}

// lookupCache は、ソース URI に紐づくキャッシュエントリを取得します。
// 旧形式（文字列を個別キーに保存）のエントリは型アサーションに失敗して
// ミス扱いになるため、キャッシュ形式の変更は安全に無視されます。
func (c *GeminiImageCore) lookupCache(sourceURI string) (cachedFile, bool) {
	val, ok := c.cache.Get(cacheKeyFileAPI + sourceURI)
	if !ok {
		return cachedFile{}, false
	}
	entry, ok := val.(cachedFile)
	if !ok || entry.URI == "" {
		return cachedFile{}, false
	}
	return entry, true
}

// storeCache は、ソース URI に紐づくキャッシュエントリを保存します。
func (c *GeminiImageCore) storeCache(sourceURI string, entry cachedFile) {
	c.cache.Set(cacheKeyFileAPI+sourceURI, entry, c.expiration)
}

// removeFromCache は、指定されたソース URI に紐づくキャッシュエントリを削除します。
func (c *GeminiImageCore) removeFromCache(sourceURI string) {
	c.cache.Delete(cacheKeyFileAPI + sourceURI)
}

// detectUploadSource は、バッファ付きリーダーに有効な画像データが含まれていることを検証し、MIMETypeを返します。
func detectUploadSource(br *bufio.Reader) (string, error) {
	head, err := br.Peek(512)
	if err != nil && !errors.Is(err, io.EOF) {
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
