package generator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// GeminiGenerator は、単一パネル生成と複数画像ページ生成の両方を担当するジェネレーターです。
type GeminiGenerator struct {
	imgCore  ImageGeneratorCore
	aiClient gemini.GenerativeModel
	model    string
}

// NewGeminiGenerator は GeminiGenerator を初期化するのだ。
func NewGeminiGenerator(
	core ImageGeneratorCore,
	aiClient gemini.GenerativeModel,
	model string,
) (*GeminiGenerator, error) {
	if core == nil || aiClient == nil {
		return nil, fmt.Errorf("必要な依存関係（core または aiClient）が不足しています")
	}

	return &GeminiGenerator{
		imgCore:  core,
		aiClient: aiClient,
		model:    model,
	}, nil
}

// GenerateMangaPanel は単一のパネル生成を行います。
func (g *GeminiGenerator) GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error) {
	// プロンプトの組み立て（ネガティブプロンプトの結合）
	finalPrompt := buildFinalPrompt(req.Prompt, req.NegativePrompt)
	parts := []*genai.Part{{Text: finalPrompt}}

	if req.ReferenceURL != "" {
		if imgPart := g.imgCore.prepareImagePart(ctx, req.ReferenceURL); imgPart != nil {
			parts = append(parts, imgPart)
		}
	}

	// generateInternal を呼び出す際、req.SystemPrompt を渡すように修正したのだ
	resp, err := g.generateInternal(ctx, parts, req.AspectRatio, req.SystemPrompt, req.Seed)
	if err != nil {
		return nil, fmt.Errorf("Geminiパネル生成エラー: %w", err)
	}
	return resp, nil
}

// GenerateMangaPage は複数画像を参照して1ページ生成を行うのだ。
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

// generateInternal は画像生成の共通ロジックを処理する内部ヘルパーなのだ。
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
		Seed:         seedToPtrInt32(seed), // 型変換ヘルパー
	}

	resp, err := g.aiClient.GenerateWithParts(ctx, g.model, parts, opts)
	if err != nil {
		return nil, err
	}

	// 戻り値の Seed 値を決定するために dereferenceSeed を使用
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

// --- 以下、ヘルパー関数 ---

// buildFinalPrompt はユーザープロンプトとネガティブプロンプトを整形するのだ。
func buildFinalPrompt(prompt, negative string) string {
	if negative == "" {
		return prompt
	}
	return fmt.Sprintf("%s\n\n[Negative Prompt]\n%s", prompt, negative)
}
