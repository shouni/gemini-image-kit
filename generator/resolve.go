package generator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/imgutil"
	"github.com/shouni/gemini-image-kit/ports"
)

// ResolveReference は、参照画像1件を送信できる添付へ解決します。
//
// 解決方法はバックエンドと URI の種類で決まります。
//
//  1. Vertex AI + gs:// — 転送せずそのまま参照します（最も安い経路）。
//  2. FileAPIURI が指定済み — 呼び出し側が既にアップロードしたものをそのまま参照します。
//  3. Gemini API — File API へアップロードし、以降は URI で参照します（キャッシュ + singleflight）。
//     同じ参照画像を繰り返し使う場合、毎回バイト列を送るより安く済みます。
//  4. Vertex AI + 非 gs:// — インライン送信します。Vertex AI には File API が無いためです。
//
// 参照先を持たない ImageURI は空の添付を返します（呼び出し側が読み飛ばせるように、
// エラーにはしません）。GeminiImageCoreConfig.InlineReferences が true の場合は
// 3 を行わず、常にインライン送信します。
func (c *GeminiImageCore) ResolveReference(ctx context.Context, uri ports.ImageURI) (gemini.Attachment, error) {
	// Vertex AI の gs:// 参照は、FileAPIURI の指定より優先する。転送が一切発生しないため。
	if c.IsVertexAI() && isGCSURI(uri.ReferenceURL) {
		return fileAttachment(uri.ReferenceURL, uri.ReferenceURL), nil
	}
	if uri.FileAPIURI != "" {
		return fileAttachment(uri.FileAPIURI, uri.ReferenceURL), nil
	}
	if uri.IsEmpty() {
		return gemini.Attachment{}, nil
	}

	if c.shouldUploadReference() {
		if attachment, ok := c.uploadedReference(ctx, uri.ReferenceURL); ok {
			return attachment, nil
		}
		// アップロードは送信量を減らすための最適化なので、失敗しても生成自体は
		// インライン送信で続行する。取得そのものが失敗していれば、そちらでも同じ理由で落ちる。
	}
	return c.PrepareImageAttachment(ctx, uri.ReferenceURL)
}

// shouldUploadReference は、参照画像を File API へ上げる経路を使うかを返します。
// Vertex AI には File API が無いため、そちらでは常にインライン送信になります。
func (c *GeminiImageCore) shouldUploadReference() bool {
	return !c.inlineReferences && !c.IsVertexAI()
}

// uploadedReference は File API 経由の参照を組み立てます。
// アップロードに失敗した場合は警告を残して false を返し、呼び出し側のフォールバックに委ねます。
func (c *GeminiImageCore) uploadedReference(ctx context.Context, referenceURL string) (gemini.Attachment, bool) {
	uploadedURI, err := c.EnsureUploaded(ctx, referenceURL)
	if err != nil {
		slog.WarnContext(ctx, "参照画像の File API へのアップロードに失敗しました。インライン送信に切り替えます",
			"reference", referenceURL, "error", err)
		return gemini.Attachment{}, false
	}
	return fileAttachment(uploadedURI, referenceURL), true
}

// PrepareImageAttachment は URL または cloud storage から画像を準備し、添付へ変換します。
func (c *GeminiImageCore) PrepareImageAttachment(ctx context.Context, rawURL string) (gemini.Attachment, error) {
	// キャッシュヒット時も非キャッシュ経路と同じ組み立てを通す。
	// ここで MIMEType を省くと、同じ画像でもキャッシュの有無で
	// API に送るペイロードが変わってしまう。
	if entry, ok := c.lookupCache(rawURL); ok {
		return fileAttachment(entry.URI, rawURL), nil
	}

	rc, err := c.fetchImageData(ctx, rawURL)
	if err != nil {
		return gemini.Attachment{}, fmt.Errorf("failed to fetch image data: %w", err)
	}
	defer rc.Close()

	rawData, err := io.ReadAll(rc)
	if err != nil {
		return gemini.Attachment{}, fmt.Errorf("failed to read image data: %w", err)
	}

	mimeType := imgutil.DetectMIMEType(rawData)
	if !imgutil.IsImageMIMEType(mimeType) {
		return gemini.Attachment{}, fmt.Errorf("%w: %s", ErrUnsupportedFileFormat, mimeType)
	}

	finalData := rawData
	if c.shouldCompress(mimeType) {
		compressed, err := imgutil.CompressToJPEG(bytes.NewReader(rawData), c.compressionQuality)
		if err != nil {
			return gemini.Attachment{}, fmt.Errorf("failed to compress image: %w", err)
		}
		finalData = compressed
		mimeType = "image/jpeg"
	}

	return gemini.Attachment{MIMEType: mimeType, Data: finalData}, nil
}
