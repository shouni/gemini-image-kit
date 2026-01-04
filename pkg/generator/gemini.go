package generator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// GeminiGenerator は単一パネルと一括ページ生成の両方を担当する統合生成器なのだ！
type GeminiGenerator struct {
	imgCore  ImageGeneratorCore
	aiClient gemini.GenerativeModel
	model    string
}

// NewGeminiGenerator は GeminiGenerator を初期化するのだ。
// 依存関係が不足している場合はエラーを返すライブラリとして正しい設計なのだ。
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

// GenerateMangaPanel は単一のパネル生成を行うのだ（ImageGenerator インターフェースの実装）。
func (g *GeminiGenerator) GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error) {
	parts := []*genai.Part{{Text: req.Prompt}}

	if req.ReferenceURL != "" {
		if imgPart := g.imgCore.PrepareImagePart(ctx, req.ReferenceURL); imgPart != nil {
			parts = append(parts, imgPart)
		}
	}

	opts := gemini.ImageOptions{
		AspectRatio: req.AspectRatio,
		Seed:        seedToPtrInt32(req.Seed),
	}

	resp, err := g.aiClient.GenerateWithParts(ctx, g.model, parts, opts)
	if err != nil {
		return nil, fmt.Errorf("Geminiパネル生成エラー: %w", err)
	}

	out, err := g.imgCore.ParseToResponse(resp, dereferenceSeed(req.Seed))
	if err != nil {
		return nil, err
	}

	return &domain.ImageResponse{
		Data:     out.Data,
		MimeType: out.MimeType,
		UsedSeed: out.UsedSeed,
	}, nil
}

// GenerateMangaPage は複数画像を参照して1ページ生成を行うのだ（MangaPageGenerator インターフェースの実装）。
func (g *GeminiGenerator) GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error) {
	slog.Info("Gemini一括生成リクエスト準備中", "model", g.model, "ref_count", len(req.ReferenceURLs))

	parts := []*genai.Part{{Text: req.Prompt}}
	for _, url := range req.ReferenceURLs {
		if url == "" {
			continue
		}
		if imgPart := g.imgCore.PrepareImagePart(ctx, url); imgPart != nil {
			parts = append(parts, imgPart)
		}
	}

	opts := gemini.ImageOptions{
		AspectRatio: req.AspectRatio,
		Seed:        seedToPtrInt32(req.Seed),
	}

	resp, err := g.aiClient.GenerateWithParts(ctx, g.model, parts, opts)
	if err != nil {
		return nil, fmt.Errorf("Gemini一括ページ生成エラー: %w", err)
	}

	out, err := g.imgCore.ParseToResponse(resp, dereferenceSeed(req.Seed))
	if err != nil {
		return nil, err
	}

	return &domain.ImageResponse{
		Data:     out.Data,
		MimeType: out.MimeType,
		UsedSeed: out.UsedSeed,
	}, nil
}
