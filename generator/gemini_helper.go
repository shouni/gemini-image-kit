package generator

import (
	"context"
	"fmt"
	"strings"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/imgutil"
	"github.com/shouni/gemini-image-kit/ports"
)

// generate は画像生成のコアロジックです。
func (g *GeminiGenerator) generate(ctx context.Context, req ports.GenerationOptions, uris []ports.ImageURI) (*ports.ImageResponse, error) {
	if req.Model == "" {
		return nil, ErrModelRequired
	}
	finalPrompt := buildFinalPrompt(req.Prompt, req.NegativePrompt)
	if finalPrompt == "" {
		return nil, ErrEmptyPrompt
	}

	// 1. 画像アセット（素材）を収集
	attachments, err := g.collectImageAttachments(ctx, uris)
	if err != nil {
		return nil, err
	}

	// 2. ImageSize を含めたオプション構築
	opts := g.toOptions(req)
	return g.core.ExecuteRequest(ctx, req.Model, finalPrompt, attachments, opts)
}

// collectImageAttachments は ImageURI 構造体から添付を生成します。
func (g *GeminiGenerator) collectImageAttachments(ctx context.Context, uris []ports.ImageURI) ([]gemini.Attachment, error) {
	attachments := make([]gemini.Attachment, 0, len(uris))

	for _, uri := range uris {
		attachment, err := g.resolveImageAttachment(ctx, uri)
		if err != nil {
			return nil, err
		}
		// 参照先を持たない要素は送るものが無いので落とす。
		if !attachment.IsEmpty() {
			attachments = append(attachments, attachment)
		}
	}
	return attachments, nil
}

// resolveImageAttachment は ImageURI から添付を生成します。
func (g *GeminiGenerator) resolveImageAttachment(ctx context.Context, uri ports.ImageURI) (gemini.Attachment, error) {
	if g.core.IsVertexAI() && IsGCSURI(uri.ReferenceURL) {
		return fileAttachment(uri.ReferenceURL, uri.ReferenceURL), nil
	}
	if uri.FileAPIURI != "" {
		return fileAttachment(uri.FileAPIURI, uri.ReferenceURL), nil
	}
	// 参照先が一切設定されていない要素は読み飛ばす（テキストのみの生成で使われる）。
	if uri.IsEmpty() {
		return gemini.Attachment{}, nil
	}
	attachment, err := g.core.PrepareImageAttachment(ctx, uri.ReferenceURL)
	if err != nil {
		return gemini.Attachment{}, fmt.Errorf("failed to prepare image attachment for %q: %w", uri.ReferenceURL, err)
	}
	return attachment, nil
}

// buildFileDataPart はファイルデータパーツを生成します。
//
// 拡張子から MIME type を判別できない場合は MIMEType を設定しません。
// 誤った型を申告するとサーバー側のデコードが失敗しうるため、
// 推測できないときはサーバーのコンテンツ判定に委ねます。
func fileAttachment(fileURI, mimeHintURI string) gemini.Attachment {
	return gemini.Attachment{URI: fileURI, MIMEType: imgutil.GuessMIMEType(mimeHintURI)}
}

// toOptions は Gemini へのリクエストオプションを構築します。
func (g *GeminiGenerator) toOptions(req ports.GenerationOptions) gemini.GenerateOptions {
	isVertex := g.core.IsVertexAI()
	opts := gemini.GenerateOptions{
		AspectRatio:     req.AspectRatio,
		ImageSize:       req.ImageSize,
		SystemPrompt:    req.SystemPrompt,
		Seed:            req.Seed,
		SafetySettings:  gemini.NewSafetySettings(safetyThreshold(isVertex)),
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: req.MaxOutputTokens,
		ThinkingBudget:  req.ThinkingBudget,
		ThinkingLevel:   req.ThinkingLevel,
	}

	// Vertex AI の場合のみ PersonGeneration を設定する
	// Gemini API (Google AI) ではこのフィールドが含まれると致命的エラーになるため
	if isVertex {
		opts.PersonGeneration = gemini.PersonGenerationAllowAll
		if req.PersonGeneration != gemini.PersonGenerationUnspecified {
			opts.PersonGeneration = req.PersonGeneration
		}
	}

	return opts
}

// safetyThreshold は、バックエンドごとの安全フィルタ閾値を返します。
// Vertex AI は OFF を受け付けないため、そちらでは BLOCK_NONE を使います。
func safetyThreshold(isVertex bool) gemini.SafetyThreshold {
	if isVertex {
		return gemini.SafetyBlockNone
	}
	return gemini.SafetyOff
}

// buildFinalPrompt はプロンプトと否定プロンプトを結合します。
func buildFinalPrompt(prompt, negative string) string {
	p := strings.TrimSpace(prompt)
	n := strings.TrimSpace(negative)

	if p == "" && n == "" {
		return ""
	}
	if n == "" {
		return p
	}

	var sb strings.Builder
	sb.WriteString(p)
	sb.WriteString(negativePromptSeparator)
	sb.WriteString(n)
	return sb.String()
}
