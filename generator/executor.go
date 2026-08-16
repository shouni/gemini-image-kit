package generator

import (
	"context"
	"fmt"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

// executeRequest はモデルを呼び出し、レスポンスをパースします。
//
// GenerateWithAttachments はプロンプトを添付より前に置きます。この順序は実際の
// カバーアート用プロンプトで images-then-prompt の形と比較測定してあり、差は
// 実行ごとのばらつきの範囲だったため、順序の切り替えノブは設けていません。
func (g *Generator) executeRequest(ctx context.Context, model string, prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions) (*ports.ImageResponse, error) {
	resp, err := g.aiClient.GenerateWithAttachments(ctx, model, prompt, attachments, opts)
	if err != nil {
		return nil, err
	}

	return parseToResponse(resp, model, prompt, dereferenceSeed(opts.Seed))
}

// parseToResponse は Gemini からのレスポンスから画像データを抽出します。
// FinishReason の検証（安全フィルターによるブロック等）は下層の go-gemini-client が行い、
// ブロック時は生成呼び出し自体がエラーを返すため、ここでは行いません。
func parseToResponse(resp *gemini.Response, model, prompt string, seed int64) (*ports.ImageResponse, error) {
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
			Model:    model,
			Prompt:   prompt,
			Usage:    resp.Usage,
		}, nil
	}

	return nil, fmt.Errorf("%w: response contains no inline image", ErrNoImageData)
}

// dereferenceSeed は *int64 を安全にデリファレンスします。nil は 0 になります。
//
// ImageResponse.UsedSeed は「リクエストで指定したシード」で、API はレスポンスに
// シードを返しません。自動採番が既定で有効なため通常は実際のシードが入りますが、
// WithoutAutoSeed を使いつつ未指定で生成すると 0 が記録されます。
func dereferenceSeed(seed *int64) int64 {
	if seed == nil {
		return 0
	}
	return *seed
}
