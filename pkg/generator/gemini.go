package generator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

const (
	// negativePromptSeparator は、ユーザープロンプトとネガティブプロンプトを区切るためのヘッダー定義です。
	negativePromptSeparator = "\n\n[Negative Prompt]\n"
)

// GeminiGenerator は、単一パネル生成と複数画像ページ生成の両方を担当するジェネレーターです。
type GeminiGenerator struct {
	imgCore  ImageGeneratorCore
	aiClient gemini.GenerativeModel
	model    string
}

// NewGeminiGenerator は GeminiGenerator を初期化し、新しいインスタンスを返します。
func NewGeminiGenerator(
	core ImageGeneratorCore,
	aiClient gemini.GenerativeModel,
	model string,
) (*GeminiGenerator, error) {
	if core == nil {
		return nil, fmt.Errorf("core (ImageGeneratorCore) is required")
	}
	if aiClient == nil {
		return nil, fmt.Errorf("aiClient (gemini.GenerativeModel) is required")
	}

	return &GeminiGenerator{
		imgCore:  core,
		aiClient: aiClient,
		model:    model,
	}, nil
}

// GenerateMangaPanel は単一のパネル生成を実行します。
func (g *GeminiGenerator) GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error) {
	// プロンプトの組み立て
	finalPrompt := buildFinalPrompt(req.Prompt, req.NegativePrompt)
	parts := []*genai.Part{{Text: finalPrompt}}

	if req.ReferenceURL != "" {
		if imgPart := g.imgCore.prepareImagePart(ctx, req.ReferenceURL); imgPart != nil {
			parts = append(parts, imgPart)
		}
	}

	resp, err := g.generateInternal(ctx, parts, req.AspectRatio, req.SystemPrompt, req.Seed)
	if err != nil {
		return nil, fmt.Errorf("Geminiパネル生成エラー: %w", err)
	}
	return resp, nil
}

// GenerateMangaPage は複数の参照画像を基に、漫画の1ページを生成します。
func (g *GeminiGenerator) GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error) {
	slog.Info("Gemini一括生成リクエスト準備中", "model", g.model, "ref_count", len(req.ReferenceURLs))

	finalPrompt := buildFinalPrompt(req.Prompt, req.NegativePrompt)
	parts := []*genai.Part{{Text: finalPrompt}}

	for _, url := range req.ReferenceURLs {
		if url == "" {
			continue
		}
		if imgPart := g.imgCore.prepareImagePart(ctx, url); imgPart != nil {
			parts = append(parts, imgPart)
		}
	}

	resp, err := g.generateInternal(ctx, parts, req.AspectRatio, req.SystemPrompt, req.Seed)
	if err != nil {
		return nil, fmt.Errorf("Gemini一括ページ生成エラー: %w", err)
	}
	return resp, nil
}

// generateInternal は画像生成リクエストの共通処理を行う内部ヘルパーです。
func (g *GeminiGenerator) generateInternal(
	ctx context.Context,
	parts []*genai.Part,
	aspectRatio string,
	systemPrompt string,
	seed *int64,
) (*domain.ImageResponse, error) {

	opts := gemini.ImageOptions{
		AspectRatio:  aspectRatio,
		SystemPrompt: systemPrompt,
		Seed:         seedToPtrInt32(seed),
	}

	resp, err := g.aiClient.GenerateWithParts(ctx, g.model, parts, opts)
	if err != nil {
		return nil, err
	}

	out, err := g.imgCore.parseToResponse(resp, dereferenceSeed(seed))
	if err != nil {
		return nil, err
	}

	return &domain.ImageResponse{
		Data:     out.Data,
		MimeType: out.MimeType,
		UsedSeed: out.UsedSeed,
	}, nil
}

// buildFinalPrompt はユーザープロンプトとネガティブプロンプトを結合し、
// AIモデルへの最終的なプロンプト文字列を構築します。
func buildFinalPrompt(prompt, negative string) string {
	if strings.TrimSpace(negative) == "" {
		return prompt
	}
	return prompt + negativePromptSeparator + negative
}
