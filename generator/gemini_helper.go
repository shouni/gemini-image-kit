package generator

import (
	"context"
	"fmt"
	"strings"

	"github.com/shouni/go-gemini-client/gemini"
	"google.golang.org/genai"

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
	parts, err := g.collectImageParts(ctx, uris)
	if err != nil {
		return nil, err
	}

	// 2. 最後にテキストプロンプトを追加
	parts = append(parts, &genai.Part{Text: finalPrompt})

	// 3. ImageSize を含めたオプション構築
	opts := g.toOptions(req)
	return g.core.ExecuteRequest(ctx, req.Model, parts, opts)
}

// collectImageParts は ImageURI 構造体からパーツを生成します。
func (g *GeminiGenerator) collectImageParts(ctx context.Context, uris []ports.ImageURI) ([]*genai.Part, error) {
	parts := make([]*genai.Part, 0, len(uris))

	for _, uri := range uris {
		part, err := g.resolveImagePart(ctx, uri)
		if err != nil {
			return nil, err
		}
		if part != nil {
			parts = append(parts, part)
		}
	}
	return parts, nil
}

// resolveImagePart は ImageURI からパーツを生成します。
func (g *GeminiGenerator) resolveImagePart(ctx context.Context, uri ports.ImageURI) (*genai.Part, error) {
	if g.core.IsVertexAI() && IsGCSURI(uri.ReferenceURL) {
		return buildFileDataPart(uri.ReferenceURL, uri.ReferenceURL), nil
	}
	if uri.FileAPIURI != "" {
		return buildFileDataPart(uri.FileAPIURI, uri.ReferenceURL), nil
	}
	if uri.ReferenceURL == "" {
		return nil, nil
	}
	part, err := g.core.PrepareImagePart(ctx, uri.ReferenceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare image part for %q: %w", uri.ReferenceURL, err)
	}
	return part, nil
}

// buildFileDataPart はファイルデータパーツを生成します。
func buildFileDataPart(fileURI, mimeHintURI string) *genai.Part {
	return &genai.Part{
		FileData: &genai.FileData{
			FileURI:  fileURI,
			MIMEType: imgutil.GuessMIMEType(mimeHintURI),
		},
	}
}

// toOptions は Gemini へのリクエストオプションを構築します。
func (g *GeminiGenerator) toOptions(req ports.GenerationOptions) gemini.GenerateOptions {
	isVertex := g.core.IsVertexAI()
	opts := gemini.GenerateOptions{
		AspectRatio:    req.AspectRatio,
		ImageSize:      req.ImageSize,
		SystemPrompt:   req.SystemPrompt,
		Seed:           req.Seed,
		SafetySettings: g.buildSafetySettings(isVertex),
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

// buildSafetySettings は安全性設定を返します。
func (g *GeminiGenerator) buildSafetySettings(isVertex bool) []*genai.SafetySetting {
	threshold := genai.HarmBlockThresholdOff

	if isVertex {
		threshold = genai.HarmBlockThresholdBlockNone
	}

	return []*genai.SafetySetting{
		{Category: genai.HarmCategoryHarassment, Threshold: threshold},
		{Category: genai.HarmCategoryHateSpeech, Threshold: threshold},
		{Category: genai.HarmCategorySexuallyExplicit, Threshold: threshold},
		{Category: genai.HarmCategoryDangerousContent, Threshold: threshold},
	}
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
