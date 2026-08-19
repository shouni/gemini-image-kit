package generator

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/shouni/gemini-image-kit/internal/imgutil"
	"github.com/shouni/gemini-image-kit/ports"
)

const (
	// DefaultCompressionQuality は、圧縮品質が指定されなかった場合のJPEG品質値です。
	DefaultCompressionQuality = 75
	// DefaultUploadTimeout は、UploadTimeout が指定されなかった場合の
	// アップロード1回あたりの制限時間です。
	DefaultUploadTimeout = 2 * time.Minute
	// DefaultFetchTimeout は、FetchTimeout が指定されなかった場合の
	// 参照画像取得1回あたりの制限時間です。
	DefaultFetchTimeout = time.Minute
	// DefaultMaxReferenceBytes は、MaxReferenceBytes が指定されなかった場合の
	// 参照画像1枚あたりのサイズ上限です。
	DefaultMaxReferenceBytes int64 = 32 << 20 // 32 MiB
)

// sourceFetcher は、参照画像の取得元（GCS / HTTP）とその上限を束ねます。
//
// 取得を伴う resolver（FetchResolver / FileAPIResolver）が共有します。取得の
// 上限（時間・バイト数）を resolver ごとに書くと、片方だけ上限が抜ける事故が起きます。
type sourceFetcher struct {
	reader     ports.ContentReader
	downloader ports.Downloader
	timeout    time.Duration
	maxBytes   int64
}

func newSourceFetcher(reader ports.ContentReader, downloader ports.Downloader, timeout time.Duration, maxBytes int64) (sourceFetcher, error) {
	if reader == nil {
		return sourceFetcher{}, ErrReaderRequired
	}
	if downloader == nil {
		return sourceFetcher{}, ErrHTTPClientRequired
	}
	if timeout <= 0 {
		timeout = DefaultFetchTimeout
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxReferenceBytes
	}
	return sourceFetcher{reader: reader, downloader: downloader, timeout: timeout, maxBytes: maxBytes}, nil
}

// open は、URL または Cloud Storage から画像データ読み込み用の Reader を返します。
// 呼び出し側は、読み込み終了後に必ず Close() を呼び出す必要があります。
func (f sourceFetcher) open(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	if isGCSURI(rawURL) {
		return f.reader.Open(ctx, rawURL)
	}
	return f.downloader.GetStream(ctx, rawURL)
}

// withTimeout は取得 1 回分の制限時間を掛けた context を返します。
func (f sourceFetcher) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if f.timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, f.timeout)
}

// readAll は参照画像を丸ごと読み込みます。
//
// 上限なしの io.ReadAll は、遅い・巨大なリモートに対してメモリと時間を際限なく
// 費やすため、時間（timeout）とバイト数（maxBytes）の両方で縛ります。
func (f sourceFetcher) readAll(ctx context.Context, rawURL string) ([]byte, error) {
	fetchCtx, cancel := f.withTimeout(ctx)
	defer cancel()

	rc, err := f.open(fetchCtx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image data: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(io.LimitReader(rc, f.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}
	if int64(len(data)) > f.maxBytes {
		return nil, fmt.Errorf("%w: reference exceeds %d bytes: %s", ErrReferenceTooLarge, f.maxBytes, rawURL)
	}
	return data, nil
}

// compression は、送信前に JPEG へ再圧縮するかどうかの方針です。
type compression struct {
	enabled bool
	quality int
}

func newCompression(enabled bool, quality int) compression {
	if quality <= 0 {
		quality = DefaultCompressionQuality
	}
	return compression{enabled: enabled, quality: quality}
}

// applies は、その MIMEType を圧縮対象とするかを返します。
func (c compression) applies(mimeType string) bool {
	return c.enabled && imgutil.IsCompressibleMimeType(mimeType)
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
// スキームの大文字小文字は区別しません。呼び出し側（アプリの設定検証）は
// remoteio.IsGCSURI で区別する側なので、このキットの方が緩くなります。向きが逆だと
// 「入力検証は通ったのに生成で落ちる」という食い違いになるため、緩い側に倒しています。
// 公開しないのは、URI スキームの述語がこのキットの関心ではないためです。
func isGCSURI(uri string) bool {
	return strings.HasPrefix(strings.ToLower(uri), "gs://")
}
