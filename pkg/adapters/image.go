package adapters

import (
	"context"
	"fmt"

	"github.com/shouni/gemini-image-kit/pkg/domain"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// ImageAdapter は漫画のパネル画像を生成するためのインターフェースです。
type ImageAdapter interface {
	GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error)
}

// GeminiImageAdapter は漫画のパネル生成を管理するアダプター層です。
type GeminiImageAdapter struct {
	imgCore     ImageGeneratorCore     // 共通ロジック保持（コンポジション）
	apiClient   gemini.GenerativeModel // 通信クライアント
	model       string                 // 使用するモデル名
	styleSuffix string                 // 共通スタイル（画風プロンプト）
}

// NewGeminiImageAdapter は GeminiImageCore と依存関係を注入して初期化します。
func NewGeminiImageAdapter(
	core ImageGeneratorCore,
	apiClient gemini.GenerativeModel,
	modelName string,
	styleSuffix string,
) (*GeminiImageAdapter, error) {
	if core == nil || apiClient == nil {
		return nil, fmt.Errorf("必要な依存関係（core または apiClient）が不足しています")
	}

	return &GeminiImageAdapter{
		imgCore:     core,
		apiClient:   apiClient,
		model:       modelName,
		styleSuffix: styleSuffix,
	}, nil
}

// GenerateMangaPanel はドメインのリクエストを Gemini API の形式に変換して実行します。
func (a *GeminiImageAdapter) GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error) {
	// 1. プロンプトの構築（ユーザー指示 + 画風サフィックス）
	fullPrompt := a.buildPrompt(req.Prompt)

	// 2. 入力パーツ（Parts）の組み立て
	parts := []*genai.Part{
		{Text: fullPrompt},
	}

	// 参照画像があれば Core の機能を使って追加
	if req.ReferenceURL != "" {
		if imgPart := a.imgCore.PrepareImagePart(ctx, req.ReferenceURL); imgPart != nil {
			parts = append(parts, imgPart)
		}
	}

	// 3. 生成オプションの設定
	// domain.Seed は *int32 型のため、gemini.ImageOptions に直接渡すことができます。
	opts := gemini.ImageOptions{
		AspectRatio: req.AspectRatio,
		Seed:        req.Seed,
	}

	// 4. 通信実行
	resp, err := a.apiClient.GenerateWithParts(ctx, a.model, parts, opts)
	if err != nil {
		return nil, fmt.Errorf("Geminiパネル生成エラー: %w", err)
	}

	// 5. Core を使ってレスポンスを解析し、ドメインモデルへマッピング
	// ※解析時に UsedSeed を int64 で返すために req.Seed を参照するのだ
	var inputSeed int64
	if req.Seed != nil {
		inputSeed = int64(*req.Seed)
	}

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

// buildPrompt は設定された画風サフィックスをプロンプトに結合します。
func (a *GeminiImageAdapter) buildPrompt(basePrompt string) string {
	if a.styleSuffix != "" {
		return fmt.Sprintf("%s, %s", basePrompt, a.styleSuffix)
	}
	return basePrompt
}
