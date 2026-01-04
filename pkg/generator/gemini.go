package generator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// GeminiGenerator は、単一パネル生成(GenerateMangaPanel)と
// 複数画像ページ生成(GenerateMangaPage)の両方を担当する統合ジェネレーターです。
type GeminiGenerator struct {
	imgCore  ImageGeneratorCore
	aiClient gemini.GenerativeModel
	model    string
}

// NewGeminiGenerator は GeminiGenerator を初期化するのだ。
func NewGeminiGenerator(
	core ImageGeneratorCore,
	aiClient gemini.GenerativeModel,
	model string,
) (*GeminiGenerator, error) {
	if core == nil {
		return nil, fmt.Errorf("core (ImageGeneratorCore) is required")
	}
	if aiClient == nil {
		return nil, fmt.Errorf("aiClient (gemini.GenerativeModel) is required")
	}

	return &GeminiGenerator{
		imgCore:  core,
		aiClient: aiClient,
		model:    model,
	}, nil
}

// generateInternal は画像生成の共通ロジック（リクエスト、通信、解析）を一括で行うヘルパーなのだ。
func (g *GeminiGenerator) generateInternal(ctx context.Context, parts []*genai.Part, aspectRatio string, seed *int64) (*domain.ImageResponse, error) {
	opts := gemini.ImageOptions{
		AspectRatio: aspectRatio,
		Seed:        seedToPtrInt32(seed),
	}

	resp, err := g.aiClient.GenerateWithParts(ctx, g.model, parts, opts)
	if err != nil {
		return nil, err // ラップは呼び出し元で行うのだ
	}

	out, err := g.imgCore.parseToResponse(resp, dereferenceSeed(seed))
	if err != nil {
		return nil, err
	}

	return &domain.ImageResponse{
		Data:     out.Data,
		MimeType: out.MimeType,
		UsedSeed: out.UsedSeed,
	}, nil
}

// GenerateMangaPanel は単一のパネル生成を行うのだ。
func (g *GeminiGenerator) GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error) {
	parts := []*genai.Part{{Text: req.Prompt}}

	if req.ReferenceURL != "" {
		if imgPart := g.imgCore.prepareImagePart(ctx, req.ReferenceURL); imgPart != nil {
			parts = append(parts, imgPart)
		}
	}

	resp, err := g.generateInternal(ctx, parts, req.AspectRatio, req.Seed)
	if err != nil {
		return nil, fmt.Errorf("Geminiパネル生成エラー: %w", err)
	}
	return resp, nil
}

// GenerateMangaPage は複数画像を参照して1ページ生成を行うのだ。
func (g *GeminiGenerator) GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error) {
	slog.Info("Gemini一括生成リクエスト準備中", "model", g.model, "ref_count", len(req.ReferenceURLs))

	parts := []*genai.Part{{Text: req.Prompt}}
	for _, url := range req.ReferenceURLs {
		if url == "" {
			continue
		}
		if imgPart := g.imgCore.prepareImagePart(ctx, url); imgPart != nil {
			parts = append(parts, imgPart)
		}
	}

	resp, err := g.generateInternal(ctx, parts, req.AspectRatio, req.Seed)
	if err != nil {
		return nil, fmt.Errorf("Gemini一括ページ生成エラー: %w", err)
	}
	return resp, nil
}
