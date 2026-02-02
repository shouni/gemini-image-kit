package generator

import (
	"context"
	"fmt"
	"strings"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
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

// generate は画像生成のコアロジックです。
func (g *GeminiGenerator) generate(ctx context.Context, model, prompt, negative string, uris []domain.ImageURI, ar, size, sp string, seed *int64) (*domain.ImageResponse, error) {
	finalPrompt := buildFinalPrompt(prompt, negative)
	if finalPrompt == "" {
		return nil, fmt.Errorf("prompt cannot be empty")
	}

	// 1. 画像アセット（素材）を収集
	parts := g.collectImageParts(ctx, uris)

	// 2. 最後にテキストプロンプトを追加
	parts = append(parts, &genai.Part{Text: finalPrompt})

	// 3. ImageSize を含めたオプション構築
	opts := g.toOptions(ar, size, sp, seed)
	return g.core.ExecuteRequest(ctx, model, parts, opts)
}

// collectImageParts は ImageURI 構造体からパーツを生成します。
func (g *GeminiGenerator) collectImageParts(ctx context.Context, uris []domain.ImageURI) []*genai.Part {
	parts := make([]*genai.Part, 0, len(uris))

	for _, uri := range uris {
		// Gemini File API URI がある場合は最優先で使用
		if uri.FileAPIURI != "" {
			parts = append(parts, &genai.Part{
				FileData: &genai.FileData{FileURI: uri.FileAPIURI},
			})
			continue
		}

		// なければ ReferenceURL からフォールバック
		if uri.ReferenceURL != "" {
			if res := g.core.PrepareImagePart(ctx, uri.ReferenceURL); res != nil {
				parts = append(parts, res)
			}
		}
	}
	return parts
}

// toOptions は Gemini へのリクエストオプションを構築します。
func (g *GeminiGenerator) toOptions(ar, size, sp string, seed *int64) gemini.GenerateOptions {
	return gemini.GenerateOptions{
		AspectRatio:  ar,
		ImageSize:    size,
		SystemPrompt: sp,
		Seed:         seed,
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
