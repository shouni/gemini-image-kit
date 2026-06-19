package generator

import (
	"context"
	"fmt"
	"math"

	"github.com/shouni/gemini-image-kit/ports"
	"google.golang.org/genai"
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

// EditImage は入力画像と任意のマスクを使って画像を編集します。
func (g *GeminiGenerator) EditImage(ctx context.Context, req ports.EditImageRequest) (*ports.ImageResponse, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	finalPrompt := buildEditPrompt(req.EditPrompt, req.TargetBBox)
	if finalPrompt == "" {
		return nil, fmt.Errorf("edit prompt cannot be empty")
	}

	source, err := g.resolveReferenceImage(ctx, req.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare source image: %w", err)
	}

	referenceImages := []genai.ReferenceImage{
		genai.NewRawReferenceImage(source, 1),
	}

	if !req.Mask.IsEmpty() {
		mask, err := g.resolveReferenceImage(ctx, req.Mask)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare mask image: %w", err)
		}
		referenceImages = append(referenceImages, genai.NewMaskReferenceImage(mask, 2, &genai.MaskReferenceConfig{
			MaskMode: genai.MaskReferenceModeMaskModeUserProvided,
		}))
	}

	seed, err := toInt32Seed(req.Seed)
	if err != nil {
		return nil, err
	}
	config := &genai.EditImageConfig{
		NumberOfImages: 1,
		Seed:           seed,
	}

	return g.core.ExecuteEditRequest(ctx, req.Model, finalPrompt, referenceImages, config, ports.DereferenceSeed(req.Seed))
}

func toInt32Seed(seed *int64) (*int32, error) {
	if seed == nil {
		return nil, nil
	}
	if *seed < math.MinInt32 || *seed > math.MaxInt32 {
		return nil, fmt.Errorf("seed %d must be within int32 range", *seed)
	}
	v := int32(*seed)
	return &v, nil
}
