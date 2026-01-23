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

// ExecuteRequest は Gemini API を呼び出し、レスポンスをパースします。(ImageExecutor インターフェース実装)
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

// PrepareImagePart は URL または cloud storageから画像を準備し、genai.Part に変換します。(ImageExecutor インターフェース実装)
func (c *GeminiImageCore) PrepareImagePart(ctx context.Context, rawURL string) *genai.Part {
	// 1. File API キャッシュチェック
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyFileAPIURI + rawURL); ok {
			if uri, ok := val.(string); ok {
				return &genai.Part{FileData: &genai.FileData{FileURI: uri}}
			}
		}
	}

	// 2. 画像の取得と圧縮
	data, err := c.fetchImageData(ctx, rawURL)
	if err != nil {
		return nil
	}

	finalData := data
	if UseImageCompression {
		if compressed, err := imgutil.CompressToJPEG(data, ImageCompressionQuality); err == nil {
			finalData = compressed
		}
	}

	return c.toPart(finalData)
}

// fetchImageData は、指定されたURLまたはcloud storageから画像データを取得します。
// URLの安全性を検証し、cloud storageまたはHTTP経由でデータをフェッチします。
func (c *GeminiImageCore) fetchImageData(ctx context.Context, rawURL string) ([]byte, error) {
	// 1. cloud storageの場合
	if remoteio.IsRemoteURI(rawURL) {
		rc, err := c.reader.Open(ctx, rawURL)
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}

	// 2. HTTP/HTTPSの場合
	// httpClient (httpkit.Client) 内部で SkipNetworkValidation フラグに基づいた
	// 安全検証が行われるため、ここではそのまま呼び出すだけでOK。
	return c.httpClient.FetchBytes(ctx, rawURL)
}

// toPart は、与えられたデータが有効な画像MIMEタイプを持つ場合に genai.Part オブジェクトへ変換します。
// 画像でない場合は nil を返します。
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

	// FinishReasonの検証: 安全フィルターによるブロックや中断を正しくハンドリングする
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
