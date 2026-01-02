package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"google.golang.org/genai"
)

// ImageGeneratorCore は画像生成のコアロジックを抽象化するのだ
type ImageGeneratorCore interface {
	PrepareImagePart(ctx context.Context, url string) *genai.Part
	ToPart(data []byte) *genai.Part
	ParseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error)
}

// ImageCacher は画像データのキャッシュ操作を抽象化するインターフェースです。
type ImageCacher interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, d time.Duration)
}

// ImageOutput はプロジェクト固有のドメインに依存しない汎用的なレスポンス構造体です。
type ImageOutput struct {
	Data     []byte
	MimeType string
	UsedSeed int64
}

// GeminiImageCore は画像生成の共通ロジックを保持するコンポーネントです。
type GeminiImageCore struct {
	httpClient httpkit.ClientInterface
	imageCache ImageCacher
	cacheTTL   time.Duration
}

// NewGeminiImageCore は依存関係を注入して GeminiImageCore のインスタンスを生成します。
func NewGeminiImageCore(httpClient httpkit.ClientInterface, imageCache ImageCacher, cacheTTL time.Duration) *GeminiImageCore {
	return &GeminiImageCore{
		httpClient: httpClient,
		imageCache: imageCache,
		cacheTTL:   cacheTTL,
	}
}

// PrepareImagePart は URL から画像を準備して genai.Part に変換します。
func (c *GeminiImageCore) PrepareImagePart(ctx context.Context, url string) *genai.Part {
	// キャッシュの確認
	if cached, found := c.imageCache.Get(url); found {
		if data, ok := cached.([]byte); ok {
			return c.ToPart(data)
		}
		// 予期せぬ型がキャッシュされていた場合に警告を出力
		slog.WarnContext(ctx, "キャッシュされたデータが []byte 型ではありません", "url", url, "type", fmt.Sprintf("%T", cached))
	}

	// 画像のダウンロード
	imgBytes, err := c.httpClient.FetchBytes(ctx, url)
	if err != nil {
		slog.WarnContext(ctx, "参照画像のダウンロードに失敗しました。テキストのみで続行します", "url", url, "error", err)
		return nil
	}

	// キャッシュに保存
	c.imageCache.Set(url, imgBytes, c.cacheTTL)
	return c.ToPart(imgBytes)
}

// ToPart はバイト列を genai.Part (InlineData) に変換します。
func (c *GeminiImageCore) ToPart(data []byte) *genai.Part {
	mimeType := http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "image/") {
		return nil
	}
	return &genai.Part{
		InlineData: &genai.Blob{
			MIMEType: mimeType,
			Data:     data,
		},
	}
}

// ParseToResponse は Gemini のレスポンスを解析して ImageOutput に変換します。
func (c *GeminiImageCore) ParseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error) {
	if resp == nil || resp.RawResponse == nil || len(resp.RawResponse.Candidates) == 0 {
		return nil, fmt.Errorf("Geminiからの有効な応答がありませんでした")
	}

	// 現在の仕様では、Geminiからの最初の候補 (Candidate) のみを利用する。
	candidate := resp.RawResponse.Candidates[0]

	// 画像パーツの探索
	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && len(part.InlineData.Data) > 0 {
				return &ImageOutput{
					Data:     part.InlineData.Data,
					MimeType: part.InlineData.MIMEType,
					UsedSeed: seed,
				}, nil
			}
		}
	}

	// プロンプトフィードバックの確認
	if feedback := resp.RawResponse.PromptFeedback; feedback != nil {
		if feedback.BlockReason != "" {
			return nil, fmt.Errorf("プロンプトがブロックされました (理由: %s)", feedback.BlockReason)
		}
	}

	// 異常終了理由の確認
	if candidate.FinishReason != genai.FinishReasonUnspecified && candidate.FinishReason != genai.FinishReasonStop {
		return nil, fmt.Errorf("画像生成が異常終了しました (理由: %s)", candidate.FinishReason)
	}

	return nil, fmt.Errorf("画像データが見つかりませんでした")
}
