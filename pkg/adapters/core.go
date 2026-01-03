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
		slog.WarnContext(ctx, "キャッシュデータが不正な型です", "url", url, "type", fmt.Sprintf("%T", cached))
	}

	// SSRF対策のバリデーション
	if safe, err := isSafeURL(url); !safe || err != nil {
		slog.WarnContext(ctx, "SSRFの可能性がある、または不正なURLをブロックしました",
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

	// 安全フィルター等によるブロックの確認
	if candidate.FinishReason != genai.FinishReasonUnspecified && candidate.FinishReason != genai.FinishReasonStop {
		return nil, fmt.Errorf("画像生成が異常終了しました (FinishReason: %s)", candidate.FinishReason)
	}

	return nil, fmt.Errorf("画像データが見つかりませんでした")
}

// seedToPtrInt32 は domain の *int64 を Gemini SDK 用の *int32 に安全に変換します。
func seedToPtrInt32(seed *int64) *int32 {
	if seed == nil {
		return nil
	}
	// int64 から int32 へ型キャストします。
	// Goの仕様により、値がint32の範囲を超える場合は上位ビットが切り捨てられますが、
	// これはシード値の再現性において期待される挙動です。
	val := int32(*seed)
	return &val
}

// isSafeURL は SSRF 対策として URL を検証します。
// 名前解決されたすべての IP アドレスに対してプライベート IP チェックを行います。
func isSafeURL(rawURL string) (bool, error) {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false, fmt.Errorf("URLパース失敗: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false, fmt.Errorf("不許可スキーム: %s", parsedURL.Scheme)
	}

	host := parsedURL.Hostname()
	var ips []net.IP

	// 1. IPアドレスが直接指定されているか確認
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		// 2. ホスト名の場合、すべての IP を取得する
		resolvedIPs, err := net.LookupIP(host)
		if err != nil {
			return false, fmt.Errorf("名前解決失敗: %w", err)
		}
		ips = resolvedIPs
	}

	if len(ips) == 0 {
		return false, fmt.Errorf("IPが見つかりません")
	}

	// すべての解決された IP を検証する
	for _, ip := range ips {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return false, fmt.Errorf("制限されたネットワークへのアクセスを検知: %s", ip.String())
		}
	}

	return true, nil
}
