package adapters

import (
	"context"
	"fmt"

	"github.com/shouni/gemini-image-kit/pkg/domain"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// ImageGenerator は漫画のパネル画像を生成するためのインターフェースです。
type ImageGenerator interface {
	GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error)
}

// GeminiImageGenerator は漫画のパネル生成を管理するアダプター層です。
type GeminiImageGenerator struct {
	imgCore     ImageGeneratorCore     // 共通ロジック保持（コンポジション）
	aiClient    gemini.GenerativeModel // 通信クライアント
	model       string                 // 使用するモデル名
	styleSuffix string                 // 共通スタイル（画風プロンプト）
}

// NewGeminiImageGenerator  は GeminiImageCore と依存関係を注入して初期化します。
func NewGeminiImageGenerator(
	core ImageGeneratorCore,
	aiClient gemini.GenerativeModel,
	modelName string,
) (*GeminiImageGenerator, error) {
	return &GeminiImageGenerator{
		imgCore:  core,
		aiClient: aiClient,
		model:    modelName,
	}, nil
}

// GenerateMangaPanel はドメインのリクエストを Gemini API の形式に変換して実行します。
func (a *GeminiImageGenerator) GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error) {
	parts := []*genai.Part{
		{Text: req.Prompt},
	}

	// 参照画像があれば Core の機能を使って追加
	if req.ReferenceURL != "" {
		if imgPart := a.imgCore.PrepareImagePart(ctx, req.ReferenceURL); imgPart != nil {
			parts = append(parts, imgPart)
		}
	}

	// 生成オプションの設定
	// domain.Seed (*int64) を SDK 用の *int32 に変換する
	opts := gemini.ImageOptions{
		AspectRatio: req.AspectRatio,
		Seed:        seedToPtrInt32(req.Seed),
	}

	// 通信実行
	resp, err := a.aiClient.GenerateWithParts(ctx, a.model, parts, opts)
	if err != nil {
		return nil, fmt.Errorf("Geminiパネル生成エラー: %w", err)
	}

	// Core を使ってレスポンスを解析し、ドメインモデルへマッピングします。
	// 入力シード値を UsedSeed の初期値として扱うため、int64 型で抽出します。
	inputSeed := dereferenceSeed(req.Seed)

	out, err := a.imgCore.ParseToResponse(resp, inputSeed)
	if err != nil {
		return nil, err
	}

	return &domain.ImageResponse{
		Data:     out.Data,
		MimeType: out.MimeType,
		UsedSeed: out.UsedSeed,
	}, nil
}
