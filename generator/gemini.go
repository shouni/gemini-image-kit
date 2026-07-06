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

// GenerateSingleImage は単一の参照画像を使って画像を生成します。
func (g *GeminiGenerator) GenerateSingleImage(ctx context.Context, req ports.SingleImageRequest) (*ports.ImageResponse, error) {
	return g.generate(
		ctx,
		req.GenerationOptions,
		[]ports.ImageURI{req.Image},
	)
}

// GenerateFusedImage は複数の参照画像を統合して1枚の画像を生成します。
func (g *GeminiGenerator) GenerateFusedImage(ctx context.Context, req ports.ImageFusionRequest) (*ports.ImageResponse, error) {
	return g.generate(
		ctx,
		req.GenerationOptions,
		req.Images,
	)
}
