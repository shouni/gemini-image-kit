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
	model string
	core  ImageExecutor // インターフェースに依存し、密結合を解消
}

// NewGeminiGenerator は新しい GeminiGenerator を作成します。
func NewGeminiGenerator(model string, core ImageExecutor) (*GeminiGenerator, error) {
	if core == nil {
		return nil, fmt.Errorf("core (ImageExecutor) is required")
	}
	return &GeminiGenerator{model: model, core: core}, nil
}

// GenerateMangaPanel は単一のパネル画像を生成します。
func (g *GeminiGenerator) GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error) {
	parts := []*genai.Part{{Text: buildFinalPrompt(req.Prompt, req.NegativePrompt)}}

	// File API URI を優先し、なければ ReferenceURL (URL/GCS) を試行
	if req.FileAPIURI != "" {
		parts = append(parts, &genai.Part{FileData: &genai.FileData{FileURI: req.FileAPIURI}})
	} else if req.ReferenceURL != "" {
		if res := g.core.PrepareImagePart(ctx, req.ReferenceURL); res != nil {
			parts = append(parts, res)
		}
	}

	opts := g.toOptions(req.AspectRatio, req.SystemPrompt, req.Seed)
	return g.core.ExecuteRequest(ctx, g.model, parts, opts)
}

// GenerateMangaPage は複数アセットを参照してページ（または複雑なパネル）画像を生成します。
func (g *GeminiGenerator) GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error) {
	finalPrompt := buildFinalPrompt(req.Prompt, req.NegativePrompt)
	if finalPrompt == "" {
		return nil, fmt.Errorf("prompt cannot be empty")
	}
	parts := []*genai.Part{{Text: finalPrompt}}

	// 有効な File API URI が追加されたかどうかをフラグで管理
	var hasValidFileAPIURI bool
	for _, uri := range req.FileAPIURIs {
		if uri != "" {
			parts = append(parts, &genai.Part{FileData: &genai.FileData{FileURI: uri}})
			hasValidFileAPIURI = true
		}
	}

	// 有効な File API URI が一つもなかった場合のみ、ReferenceURLs にフォールバック
	if !hasValidFileAPIURI {
		for _, url := range req.ReferenceURLs {
			if url != "" {
				if res := g.core.PrepareImagePart(ctx, url); res != nil {
					parts = append(parts, res)
				}
			}
		}
	}

	opts := g.toOptions(req.AspectRatio, req.SystemPrompt, req.Seed)
	return g.core.ExecuteRequest(ctx, g.model, parts, opts)
}

// toOptions 提供されたアスペクト比、システム プロンプト、シード値を GenerateOptions 構造体に変換します。
func (g *GeminiGenerator) toOptions(ar, sp string, seed *int64) gemini.GenerateOptions {
	return gemini.GenerateOptions{AspectRatio: ar, SystemPrompt: sp, Seed: seed}
}

// buildFinalPrompt はプロンプトとネガティブプロンプトを安全に結合します。
func buildFinalPrompt(prompt, negative string) string {
	p := strings.TrimSpace(prompt)
	n := strings.TrimSpace(negative)

	if p == "" && n == "" {
		return ""
	}
	if n == "" {
		return p
	}

	// strings.Builder を使用して効率的かつ明示的に構築
	var sb strings.Builder
	sb.WriteString(p)
	sb.WriteString(negativePromptSeparator)
	sb.WriteString(n)
	return sb.String()
}
