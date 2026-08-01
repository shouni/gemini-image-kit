package generator

import (
	"context"
	"fmt"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

// ExecuteRequest は Gemini API を呼び出し、レスポンスをパースします。
func (c *GeminiImageCore) ExecuteRequest(ctx context.Context, model string, prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions) (*ports.ImageResponse, error) {
	resp, err := c.aiClient.GenerateWithAttachments(ctx, model, prompt, attachments, opts)
	if err != nil {
		return nil, err
	}

	return c.parseToResponse(resp, dereferenceSeed(opts.Seed))
}

// parseToResponse は Gemini からのレスポンスから画像データを抽出します。
// FinishReason の検証（安全フィルターによるブロック等）は下層の go-gemini-client が行い、
// ブロック時は生成呼び出し自体がエラーを返すため、ここでは行いません。
func (c *GeminiImageCore) parseToResponse(resp *gemini.Response, seed int64) (*ports.ImageResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("%w: no response", ErrNoImageData)
	}

	// Response.Attachments は MIME type 込みで返るため、保存時の Content-Type を決めるために
	// 生の SDK レスポンスを辿る必要はありません。
	for _, attachment := range resp.Attachments {
		if len(attachment.Data) == 0 {
			continue
		}
		return &ports.ImageResponse{
			Data:     attachment.Data,
			MimeType: attachment.MIMEType,
			UsedSeed: seed,
		}, nil
	}

	return nil, fmt.Errorf("%w: response contains no inline image", ErrNoImageData)
}

// dereferenceSeed は *int64 を安全にデリファレンスします。nil は 0 になります。
//
// ImageResponse.UsedSeed は「リクエストで指定したシード」で、API はレスポンスに
// シードを返しません。未指定（nil）のまま生成すると 0 が記録されるため、
// 再現性が必要なら WithAutoSeed を使ってください。
func dereferenceSeed(seed *int64) int64 {
	if seed == nil {
		return 0
	}
	return *seed
}
