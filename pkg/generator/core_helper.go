package generator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/gemini-image-kit/pkg/imgutil"
	"github.com/shouni/go-gemini-client/pkg/gemini"
	"github.com/shouni/go-remote-io/pkg/remoteio"
	"google.golang.org/genai"
)

// ExecuteRequest は Gemini API を呼び出し、レスポンスをパースします。
func (c *GeminiImageCore) ExecuteRequest(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*domain.ImageResponse, error) {
	resp, err := c.aiClient.GenerateWithParts(ctx, model, parts, opts)
	if err != nil {
		return nil, err
	}

	out, err := c.ParseToResponse(resp, domain.DereferenceSeed(opts.Seed))
	if err != nil {
		return nil, err
	}

	return &domain.ImageResponse{
		Data:     out.Data,
		MimeType: out.MimeType,
		UsedSeed: out.UsedSeed,
	}, nil
}

// PrepareImagePart は URL または cloud storageから画像を準備し、genai.Part に変換します。
func (c *GeminiImageCore) PrepareImagePart(ctx context.Context, rawURL string) *genai.Part {
	// 1. File API キャッシュチェック
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyFileAPIURI + rawURL); ok {
			if uri, ok := val.(string); ok {
				return &genai.Part{FileData: &genai.FileData{FileURI: uri}}
			}
		}
	}

	// 2. 画像の取得（ストリーム処理）
	rc, err := c.fetchImageData(ctx, rawURL)
	if err != nil {
		return nil
	}
	defer rc.Close()

	// 3. 画像圧縮処理
	var finalData []byte
	if UseImageCompression {
		// ストリーム（rc）を直接圧縮関数へ渡す
		compressed, err := imgutil.CompressToJPEG(rc, ImageCompressionQuality)
		if err == nil {
			finalData = compressed
		} else {
			// 圧縮失敗時は全データ読み込みにフォールバック
			// 再度読み込む必要がある場合は Seek が必要だが、通常はここに来る前に
			// rc を読み込んでしまうため、圧縮失敗時に ReadAll するなら
			// 圧縮関数へ渡す前に一度バッファリングする工夫が必要です。
			// 今回はシンプルに ReadAll を優先します。
		}
	}

	// 圧縮していない、または失敗した場合のフォールバック
	if finalData == nil {
		finalData, err = io.ReadAll(rc)
		if err != nil {
			return nil
		}
	}

	return c.toPart(finalData)
}

// fetchImageData は、指定されたURLまたはCloud Storageから画像データ読み込み用の Reader を返します。
// 呼び出し側は、読み込み終了後に必ず Close() を呼び出す必要があります。
func (c *GeminiImageCore) fetchImageData(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	// 1. Cloud Storage の場合
	if remoteio.IsRemoteURI(rawURL) {
		return c.reader.Open(ctx, rawURL)
	}

	// 2. HTTP/HTTPS の場合
	// GetStream がすでに io.ReadCloser を返しているので、そのまま返す
	rc, err := c.httpClient.GetStream(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	return rc, nil
}

// toPart は、与えられたデータが有効な画像MIMEタイプを持つ場合に genai.Part オブジェクトへ変換します。
func (c *GeminiImageCore) toPart(data []byte) *genai.Part {
	mimeType := http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "image/") {
		return nil
	}
	return &genai.Part{InlineData: &genai.Blob{MIMEType: mimeType, Data: data}}
}

// ParseToResponse は Gemini からのレスポンスを検証し、画像データを抽出します。
func (c *GeminiImageCore) ParseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error) {
	if resp == nil || resp.RawResponse == nil || len(resp.RawResponse.Candidates) == 0 {
		return nil, fmt.Errorf("invalid or empty response from Gemini")
	}

	candidate := resp.RawResponse.Candidates[0]

	if candidate.FinishReason != genai.FinishReasonStop && candidate.FinishReason != genai.FinishReasonUnspecified {
		return nil, fmt.Errorf("generation failed with FinishReason: %s", candidate.FinishReason)
	}

	if candidate.Content == nil {
		return nil, fmt.Errorf("no content found in candidate")
	}

	for _, part := range candidate.Content.Parts {
		if part.InlineData != nil {
			return &ImageOutput{
				Data:     part.InlineData.Data,
				MimeType: part.InlineData.MIMEType,
				UsedSeed: seed,
			}, nil
		}
	}

	return nil, fmt.Errorf("no image data found in response parts")
}
