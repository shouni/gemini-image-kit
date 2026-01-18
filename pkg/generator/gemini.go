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

type GeminiGenerator struct {
	model string
	core  *GeminiImageCore // インターフェースではなく具象型にする
}

func NewGeminiGenerator(model string, core *GeminiImageCore) (*GeminiGenerator, error) {
	if core == nil {
		return nil, fmt.Errorf("core is required")
	}
	return &GeminiGenerator{model: model, core: core}, nil
}

func (g *GeminiGenerator) GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error) {
	parts := []*genai.Part{{Text: buildFinalPrompt(req.Prompt, req.NegativePrompt)}}

	if req.FileAPIURI != "" {
		parts = append(parts, &genai.Part{FileData: &genai.FileData{FileURI: req.FileAPIURI}})
	} else if req.ReferenceURL != "" {
		res := g.core.prepareImagePart(ctx, req.ReferenceURL)
		parts = append(parts, res)
	}

	opts := g.toOptions(req.AspectRatio, req.SystemPrompt, req.Seed)
	return g.core.executeRequest(ctx, g.model, parts, opts)
}

func (g *GeminiGenerator) GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error) {
	parts := []*genai.Part{{Text: buildFinalPrompt(req.Prompt, req.NegativePrompt)}}

	// File API URI を優先
	for _, uri := range req.FileAPIURIs {
		if uri != "" {
			parts = append(parts, &genai.Part{FileData: &genai.FileData{FileURI: uri}})
		}
	}

	// URIがない場合のみフォールバック
	if len(req.FileAPIURIs) == 0 {
		for _, url := range req.ReferenceURLs {
			res := g.core.prepareImagePart(ctx, url)
			parts = append(parts, res)
		}
	}

	opts := g.toOptions(req.AspectRatio, req.SystemPrompt, req.Seed)
	return g.core.executeRequest(ctx, g.model, parts, opts)
}

func (g *GeminiGenerator) toOptions(ar, sp string, seed *int64) gemini.GenerateOptions {
	return gemini.GenerateOptions{AspectRatio: ar, SystemPrompt: sp, Seed: seed}
}

func buildFinalPrompt(prompt, negative string) string {
	p, n := strings.TrimSpace(prompt), strings.TrimSpace(negative)
	if n == "" {
		return p
	}
	return p + negativePromptSeparator + n
}
