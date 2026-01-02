package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"google.golang.org/genai"
)

// ImageGeneratorCore は画像生成のコアロジックを抽象化するインターフェースです。
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

	// SSRF対策のバリデーション
	if safe, err := isSafeURL(url); !safe || err != nil {
		slog.WarnContext(ctx, "SSRFの可能性がある、または不正なURLを検知したためブロックしました",
			"url", url, "error", err)
		return nil
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
		slog.Warn("MIMEタイプが画像ではないためPartに変換できませんでした", "detected_mime_type", mimeType)
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

// seedToInt64 は *int32 型のシード値を安全に int64 型へ変換するヘルパー関数です。
// SDK互換の型(*int32)とドメイン層で扱う型(int64)の差異を吸収します。
func seedToInt64(seed *int32) int64 {
	if seed != nil {
		return int64(*seed)
	}
	// 指定なしの場合は 0 を返す。
	// ※ 0 が有効なシード値として扱われるか、ランダム扱いになるかは Gemini API の仕様に準拠するのだ。
	return 0
}

// isSafeURL は SSRF 対策として、URL がパブリックなものであるかを検証するのだ。
func isSafeURL(rawURL string) (bool, error) {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false, fmt.Errorf("URLのパースに失敗しました: %w", err)
	}

	// スキームの制限（http/https のみ）
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false, fmt.Errorf("許可されていないスキームです: %s", parsedURL.Scheme)
	}

	host := parsedURL.Hostname()
	// ホスト名がIPアドレスか確認
	ip := net.ParseIP(host)
	if ip == nil {
		// ホスト名の場合、名前解決して検証するのだ
		ips, err := net.LookupIP(host)
		if err != nil {
			return false, fmt.Errorf("ホストの名前解決に失敗しました: %w", err)
		}
		if len(ips) == 0 {
			return false, fmt.Errorf("IPアドレスが見つかりませんでした")
		}
		ip = ips[0]
	}

	// プライベートIP、ループバック、リンクローカルを遮断するのだ！
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false, fmt.Errorf("プライベートまたは制限されたネットワークへのアクセスは禁止されています: %s", ip.String())
	}

	return true, nil
}
