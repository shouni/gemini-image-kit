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

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/internal/imgutil"
)

// fetchImageData は、指定されたURLまたはCloud Storageから画像データ読み込み用の Reader を返します。
// 呼び出し側は、読み込み終了後に必ず Close() を呼び出す必要があります。
func (c *GeminiImageCore) fetchImageData(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	// 1. Cloud Storage の場合
	if isGCSURI(rawURL) {
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

// isGCSURI は、指定された URI が GCS（Google Cloud Storage）を指すかを判定します。
//
// 公開していないのは、これが URI スキームの述語であってこのキットの関心ではないためです。
// 同じ判定は go-remote-io / go-utils が公開しており、利用側はそちらを使ってください。
func isGCSURI(uri string) bool {
	const prefixGCS = "gs://"
	return strings.HasPrefix(uri, prefixGCS)
}
