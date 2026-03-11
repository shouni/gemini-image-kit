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
	core         domain.ImageExecutor
}

// NewGeminiGenerator は新しい GeminiGenerator を作成します。
func NewGeminiGenerator(model, qualityModel string, core domain.ImageExecutor) (*GeminiGenerator, error) {
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

// IsVertexAI は、Vertex AI バックエンドを使用しているかを確認します。
func (g *GeminiGenerator) IsVertexAI() bool {
	return g.core.IsVertexAI()
}

// GenerateMangaPanel は単一のパネル画像を生成します。
func (g *GeminiGenerator) GenerateMangaPanel(ctx context.Context, req domain.ImagePanelRequest) (*domain.ImageResponse, error) {
	return g.generate(
		ctx,
		g.model,
		req.Options,
		[]domain.ImageURI{req.Image},
	)
}

// GenerateMangaPage は複数アセットを参照してページ画像を生成します。
func (g *GeminiGenerator) GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error) {
	return g.generate(
		ctx,
		g.qualityModel,
		req.Options,
		req.Images,
	)
}
