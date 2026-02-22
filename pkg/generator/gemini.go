package generator

import (
	"context"
	"fmt"

	"github.com/shouni/gemini-image-kit/pkg/domain"
)

const negativePromptSeparator = "\n\n[Negative Prompt]\n"

// GeminiGenerator は高レベルな画像生成ロジックを担当します。
type GeminiGenerator struct {
	model        string
	qualityModel string
	core         ImageExecutor
}

// NewGeminiGenerator は新しい GeminiGenerator を作成します。
func NewGeminiGenerator(model, qualityModel string, core ImageExecutor) (*GeminiGenerator, error) {
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if qualityModel == "" {
		return nil, fmt.Errorf("qualityModel is required")
	}
	if core == nil {
		return nil, fmt.Errorf("core (ImageExecutor) is required")
	}

	return &GeminiGenerator{
		model:        model,
		qualityModel: qualityModel,
		core:         core,
	}, nil
}

// GenerateMangaPanel は単一のパネル画像を生成します。
func (g *GeminiGenerator) GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error) {
	return g.generate(
		ctx,
		g.model,
		req.Prompt,
		req.NegativePrompt,
		[]domain.ImageURI{req.Image},
		req.AspectRatio,
		req.ImageSize,
		req.SystemPrompt,
		req.Seed,
	)
}

// GenerateMangaPage は複数アセットを参照してページ画像を生成します。
func (g *GeminiGenerator) GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error) {
	return g.generate(
		ctx,
		g.qualityModel,
		req.Prompt,
		req.NegativePrompt,
		req.Images,
		req.AspectRatio,
		req.ImageSize,
		req.SystemPrompt,
		req.Seed,
	)
}
