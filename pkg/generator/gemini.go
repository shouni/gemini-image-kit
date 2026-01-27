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
	core  ImageExecutor
}

func NewGeminiGenerator(model string, core ImageExecutor) (*GeminiGenerator, error) {
	if core == nil {
		return nil, fmt.Errorf("core (ImageExecutor) is required")
	}
	return &GeminiGenerator{model: model, core: core}, nil
}

// GenerateMangaPanel は単一のパネル画像を生成します。
func (g *GeminiGenerator) GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error) {
	// collectImageParts 側で空文字チェックを行うため、直接スライス化して渡す（簡潔化）
	return g.generate(
		ctx,
		req.Prompt,
		req.NegativePrompt,
		[]string{req.FileAPIURI},
		[]string{req.ReferenceURL},
		req.AspectRatio,
		req.SystemPrompt,
		req.Seed,
	)
}

// GenerateMangaPage は複数アセットを参照してページ画像を生成します。
func (g *GeminiGenerator) GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error) {
	return g.generate(
		ctx,
		req.Prompt,
		req.NegativePrompt,
		req.FileAPIURIs,
		req.ReferenceURLs,
		req.AspectRatio,
		req.SystemPrompt,
		req.Seed,
	)
}

// generate は画像生成のコアロジックです。
func (g *GeminiGenerator) generate(ctx context.Context, prompt, negative string, fileURIs, refURLs []string, ar, sp string, seed *int64) (*domain.ImageResponse, error) {
	finalPrompt := buildFinalPrompt(prompt, negative)
	if finalPrompt == "" {
		return nil, fmt.Errorf("prompt cannot be empty")
	}

	// 1. 画像アセット（素材）を先に収集
	parts := g.collectImageParts(ctx, fileURIs, refURLs)

	// 2. 最後にテキストプロンプト（指示）を追加（高度な合成向けの意図的な順序）
	parts = append(parts, &genai.Part{Text: finalPrompt})

	opts := g.toOptions(ar, sp, seed)
	return g.core.ExecuteRequest(ctx, g.model, parts, opts)
}

// collectImageParts はアセットからパーツを生成します。
func (g *GeminiGenerator) collectImageParts(ctx context.Context, fileURIs, refURLs []string) []*genai.Part {
	maxLen := len(fileURIs)
	if len(refURLs) > maxLen {
		maxLen = len(refURLs)
	}
	parts := make([]*genai.Part, 0, maxLen)

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

// toOptions は指定されたアスペクト比、システム プロンプト、シードを使用して gemini.GenerateOptions インスタンスを構築して返します。
func (g *GeminiGenerator) toOptions(ar, sp string, seed *int64) gemini.GenerateOptions {
	return gemini.GenerateOptions{AspectRatio: ar, SystemPrompt: sp, Seed: seed}
}

// buildFinalPrompt スはペースを削除した後、プロンプトと否定プロンプトを定義済みの文字列で区切って結合します。
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
