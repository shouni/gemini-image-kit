package generator

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/internal/imgutil"
	"github.com/shouni/gemini-image-kit/ports"
)

// resolveReference は、参照画像1件を送信できる添付へ解決します。
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
func (c *GeminiImageCore) resolveReference(ctx context.Context, uri ports.ImageURI) (gemini.Attachment, error) {
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
		attachment, ok, err := c.uploadedReference(ctx, uri.ReferenceURL)
		if err != nil {
			return gemini.Attachment{}, err
		}
		if ok {
			return attachment, nil
		}
		// アップロードは送信量を減らすための最適化なので、失敗しても生成自体は
		// インライン送信で続行する。取得そのものが失敗していれば、そちらでも同じ理由で落ちる。
	}
	return c.prepareImageAttachment(ctx, uri.ReferenceURL)
}

// shouldUploadReference は、参照画像を File API へ上げる経路を使うかを返します。
// Vertex AI には File API が無いため、そちらでは常にインライン送信になります。
func (c *GeminiImageCore) shouldUploadReference() bool {
	return !c.inlineReferences && !c.IsVertexAI()
}

// uploadedReference は File API 経由の参照を組み立てます。
//
// アップロードに失敗した場合は警告を残して ok=false を返し、呼び出し側の
// インライン送信フォールバックに委ねます。ただし失敗の原因が呼び出し側の
// キャンセルである場合はエラーを返します — 以前はキャンセルでも「アップロード
// 失敗」と誤って警告した上でフォールバックし、同じ画像をもう一度フェッチして
// 同じキャンセルで落ちていました（無駄な二重フェッチ + 紛らわしい警告）。
func (c *GeminiImageCore) uploadedReference(ctx context.Context, referenceURL string) (gemini.Attachment, bool, error) {
	uploadedURI, err := c.ensureUploaded(ctx, referenceURL)
	if err != nil {
		if ctx.Err() != nil {
			return gemini.Attachment{}, false, ctx.Err()
		}
		c.log().WarnContext(ctx, "参照画像の File API へのアップロードに失敗しました。インライン送信に切り替えます",
			"reference", referenceURL, "error", err)
		return gemini.Attachment{}, false, nil
	}
	return fileAttachment(uploadedURI, referenceURL), true, nil
}

// prepareImageAttachment は URL または cloud storage から画像を取得し、添付へ変換します。
//
// 取得には FetchTimeout と MaxReferenceBytes の上限が掛かります。上限なしの
// io.ReadAll は、遅い・巨大なリモートに対してメモリと時間を際限なく費やすためです。
func (c *GeminiImageCore) prepareImageAttachment(ctx context.Context, rawURL string) (gemini.Attachment, error) {
	// キャッシュヒット時も非キャッシュ経路と同じ組み立てを通す。
	// ここで MIMEType を省くと、同じ画像でもキャッシュの有無で
	// API に送るペイロードが変わってしまう。
	if uri, ok := c.lookupCache(rawURL); ok {
		return fileAttachment(uri, rawURL), nil
	}

	fetchCtx := ctx
	if c.fetchTimeout > 0 {
		var cancel context.CancelFunc
		fetchCtx, cancel = context.WithTimeout(ctx, c.fetchTimeout)
		defer cancel()
	}

	rc, err := c.fetchImageData(fetchCtx, rawURL)
	if err != nil {
		return gemini.Attachment{}, fmt.Errorf("failed to fetch image data: %w", err)
	}
	defer rc.Close()

	maxBytes := c.maxReferenceBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxReferenceBytes
	}
	rawData, err := io.ReadAll(io.LimitReader(rc, maxBytes+1))
	if err != nil {
		return gemini.Attachment{}, fmt.Errorf("failed to read image data: %w", err)
	}
	if int64(len(rawData)) > maxBytes {
		return gemini.Attachment{}, fmt.Errorf("%w: reference exceeds %d bytes: %s", ErrReferenceTooLarge, maxBytes, rawURL)
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
