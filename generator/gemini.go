package generator

import (
	"context"
	"fmt"

	"github.com/shouni/gemini-image-kit/ports"
)

const negativePromptSeparator = "\n\n[Negative Prompt]\n"

// GeminiGenerator は高レベルな画像生成ロジックを担当します。
type GeminiGenerator struct {
	core ports.ImageExecutor
}

// NewGeminiGenerator は新しい GeminiGenerator を作成します。
func NewGeminiGenerator(core ports.ImageExecutor) (*GeminiGenerator, error) {
	if core == nil {
		return nil, fmt.Errorf("core (ImageExecutor) is required")
	}

	return &GeminiGenerator{
		core: core,
	}, nil
}

// IsVertexAI は、Vertex AI バックエンドを使用しているかを確認します。
func (g *GeminiGenerator) IsVertexAI() bool {
	return g.core.IsVertexAI()
}

// GenerateMangaPanel は単一のパネル画像を生成します。
func (g *GeminiGenerator) GenerateMangaPanel(ctx context.Context, req ports.ImagePanelRequest) (*ports.ImageResponse, error) {
	return g.generate(
		ctx,
		req.GenerationOptions,
		[]ports.ImageURI{req.Image},
	)
}

// GenerateMangaPage は複数アセットを参照してページ画像を生成します。
func (g *GeminiGenerator) GenerateMangaPage(ctx context.Context, req ports.ImagePageRequest) (*ports.ImageResponse, error) {
	return g.generate(
		ctx,
		req.GenerationOptions,
		req.Images,
	)
}
