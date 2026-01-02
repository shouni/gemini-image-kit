package adapters

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shouni/gemini-image-kit/pkg/domain"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// GeminiMangaPageAdapter は、Geminiを利用してマンガのページ画像を生成するアダプターです。
// ImageGeneratorCore（画像処理）と aiClient（Gemini通信）を組み合わせて動作します。
type GeminiMangaPageAdapter struct {
	imgCore  ImageGeneratorCore
	aiClient gemini.GenerativeModel
	model    string
}

// NewGeminiMangaPageAdapter は、依存関係を注入してアダプターのインスタンスを作成する。
func NewGeminiMangaPageAdapter(core ImageGeneratorCore, aiClient gemini.GenerativeModel, model string) *GeminiMangaPageAdapter {
	return &GeminiMangaPageAdapter{
		imgCore:  core,
		aiClient: aiClient,
		model:    model,
	}
}

// GenerateMangaPage は、プロンプトと複数の参照画像URLを受け取り、マンガの1ページを生成します。
// 内部で画像のダウンロード、キャッシュ、Geminiへのリクエスト、レスポンスのパースを一括で行います。
func (a *GeminiMangaPageAdapter) GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error) {
	slog.Info("Gemini一括生成リクエストを準備中", "model", a.model, "ref_count", len(req.ReferenceURLs))

	// 1. プロンプト（テキストパーツ）の組み立て
	// リクエストに含まれるテキストを最初のパーツとしてセットする。
	parts := []*genai.Part{
		{Text: req.Prompt},
	}

	// 2. 複数の参照画像をすべてパーツに追加
	// URLをループで回して、imgCoreを使って画像データ（InlineData）に変換していく。
	imageCount := 0
	for i, url := range req.ReferenceURLs {
		if url == "" {
			continue
		}

		// キャッシュ確認とダウンロードを Core に委譲するのだ。
		imgPart := a.imgCore.PrepareImagePart(ctx, url)
		if imgPart == nil {
			// 失敗しても生成自体は続行し、警告ログを残すのだ。
			slog.WarnContext(ctx, "参照画像の読み込みに失敗しました", "index", i, "url", url)
			continue
		}

		parts = append(parts, imgPart)
		imageCount++
	}

	slog.Info("AIに送信するパーツ構成が完了したのだ", "total_parts", len(parts), "images", imageCount)

	// 3. 生成オプションの設定
	// アスペクト比やシード値など、生成時のパラメータをセットするのだ。
	opts := gemini.ImageOptions{
		AspectRatio: req.AspectRatio,
		Seed:        req.Seed,
	}

	// 4. Geminiクライアント経由で生成実行
	// 組み立てたパーツ群（テキスト + 画像バイナリ）をGeminiにリクエストします
	slog.Info("Geminiに画像生成をリクエストします")
	resp, err := a.aiClient.GenerateWithParts(ctx, a.model, parts, opts)
	if err != nil {
		return nil, fmt.Errorf("Geminiでの一括ページ生成に失敗しました: %w", err)
	}

	// 5. レスポンスの解析
	// Geminiから返ってきた複雑なレスポンスから、画像バイナリだけを抽出するのだ。
	inputSeed := seedToInt64(req.Seed)
	out, err := a.imgCore.ParseToResponse(resp, inputSeed)
	if err != nil {
		return nil, fmt.Errorf("レスポンスパースに失敗しました: %w", err)
	}

	// 最後にドメイン層が扱いやすい ImageResponse 型に変換して返却するのだ！
	return &domain.ImageResponse{
		Data:     out.Data,
		MimeType: out.MimeType,
		UsedSeed: out.UsedSeed,
	}, nil
}
