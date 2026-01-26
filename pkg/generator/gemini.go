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
	// 単一のリクエストをスライスに変換して共通ロジックに渡す
	fileURIs := []string{}
	if req.FileAPIURI != "" {
		fileURIs = append(fileURIs, req.FileAPIURI)
	}
	refURLs := []string{}
	if req.ReferenceURL != "" {
		refURLs = append(refURLs, req.ReferenceURL)
	}

	return g.generate(ctx, req.Prompt, req.NegativePrompt, fileURIs, refURLs, req.AspectRatio, req.SystemPrompt, req.Seed)
}

// GenerateMangaPage は複数アセットを参照してページ（または複雑なパネル）画像を生成します。
func (g *GeminiGenerator) GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error) {
	return g.generate(ctx, req.Prompt, req.NegativePrompt, req.FileAPIURIs, req.ReferenceURLs, req.AspectRatio, req.SystemPrompt, req.Seed)
}

// generate は画像生成のコアロジックをカプセル化した内部メソッドです。
func (g *GeminiGenerator) generate(ctx context.Context, prompt, negative string, fileURIs, refURLs []string, ar, sp string, seed *int64) (*domain.ImageResponse, error) {
	finalPrompt := buildFinalPrompt(prompt, negative)
	if finalPrompt == "" {
		return nil, fmt.Errorf("prompt cannot be empty")
	}

	// 1. 画像アセット（素材）を先に収集
	parts := g.collectImageParts(ctx, fileURIs, refURLs)

	// 2. 最後にテキストプロンプト（指示）を追加
	parts = append(parts, &genai.Part{Text: finalPrompt})

	// 3. 実行
	opts := g.toOptions(ar, sp, seed)
	return g.core.ExecuteRequest(ctx, g.model, parts, opts)
}

// collectImageParts は File API または ReferenceURL からパーツを生成します。
func (g *GeminiGenerator) collectImageParts(ctx context.Context, fileURIs, refURLs []string) []*genai.Part {
	var parts []*genai.Part

	// File API URI を優先
	for _, uri := range fileURIs {
		if uri != "" {
			parts = append(parts, &genai.Part{FileData: &genai.FileData{FileURI: uri}})
		}
	}

	// File API が一つもなかった場合のみ ReferenceURL を処理
	if len(parts) == 0 {
		for _, url := range refURLs {
			if url != "" {
				if res := g.core.PrepareImagePart(ctx, url); res != nil {
					parts = append(parts, res)
				}
			}
		}
	}
	return parts
}

// toOptions は、引数を基に gemini.GenerateOptions を生成します。
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

	var sb strings.Builder
	sb.WriteString(p)
	sb.WriteString(negativePromptSeparator)
	sb.WriteString(n)
	return sb.String()
}
