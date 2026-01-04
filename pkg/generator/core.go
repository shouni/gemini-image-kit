package generator

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
	"google.golang.org/genai"
)

// ImageOutput は Core が解析した結果を保持する内部構造体なのだ。
type ImageOutput struct {
	Data     []byte
	MimeType string
	UsedSeed int64
}

// HTTPClient は画像取得に必要な最小限のインターフェースなのだ。
type HTTPClient interface {
	FetchBytes(ctx context.Context, url string) ([]byte, error)
}

// ImageCacher は画像のキャッシュを担当するインターフェースなのだ。
type ImageCacher interface {
	Get(key string) (any, bool)
	Set(key string, value any, d time.Duration)
}

type GeminiImageCore struct {
	httpClient HTTPClient
	cache      ImageCacher
	expiration time.Duration
}

func NewGeminiImageCore(client HTTPClient, cache ImageCacher, exp time.Duration) *GeminiImageCore {
	return &GeminiImageCore{
		httpClient: client,
		cache:      cache,
		expiration: exp,
	}
}

// PrepareImagePart は URL から画像を準備し、Gemini 用の Part に変換するのだ。
func (c *GeminiImageCore) PrepareImagePart(ctx context.Context, url string) *genai.Part {
	// 1. キャッシュチェック
	if c.cache != nil {
		if val, ok := c.cache.Get(url); ok {
			if data, ok := val.([]byte); ok {
				return c.ToPart(data)
			}
			slog.WarnContext(ctx, "Invalid data type in image cache", "url", url, "type", fmt.Sprintf("%T", val))
		}
	}

	// 2. SSRF対策（名前解決レベルでの安全チェック）
	if safe, err := isSafeURL(url); !safe {
		slog.WarnContext(ctx, "SSRFの可能性がある、または不正なURLをブロックしました", "url", url, "error", err)
		return nil
	}

	// 3. ダウンロード
	data, err := c.httpClient.FetchBytes(ctx, url)
	if err != nil {
		slog.ErrorContext(ctx, "画像ダウンロード失敗", "url", url, "error", err)
		return nil
	}

	// 4. キャッシュ保存
	if c.cache != nil {
		c.cache.Set(url, data, c.expiration)
	}

	return c.ToPart(data)
}

// ToPart はバイナリデータを MIME タイプ付きの Part に変換するのだ。
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

// ParseToResponse は Gemini のレスポンスから画像データを抽出するのだ。
func (c *GeminiImageCore) ParseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error) {
	if resp == nil || resp.RawResponse == nil {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	raw := resp.RawResponse
	if len(raw.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	candidate := raw.Candidates[0]
	if candidate.FinishReason != genai.FinishReasonStop && candidate.FinishReason != genai.FinishReasonUnspecified {
		return nil, fmt.Errorf("generation failed with FinishReason: %s", candidate.FinishReason)
	}

	for _, part := range candidate.Content.Parts {
		if part.InlineData != nil {
			return &ImageOutput{
				Data:     part.InlineData.Data,
				MimeType: part.InlineData.MIMEType,
				UsedSeed: seed,
			}, nil
		}
	}

	return nil, fmt.Errorf("no image data found in response parts")
}

// isSafeURL は SSRF 対策として URL を検証するのだ。
func isSafeURL(rawURL string) (bool, error) {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false, fmt.Errorf("URLパース失敗: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false, fmt.Errorf("不許可スキーム: %s", parsedURL.Scheme)
	}

	ips, err := net.LookupIP(parsedURL.Hostname())
	if err != nil {
		return false, fmt.Errorf("名前解決失敗: %w", err)
	}

	if len(ips) == 0 {
		return false, fmt.Errorf("IPアドレスが見つかりません")
	}

	for _, ip := range ips {
		// プライベート、ループバック、リンクローカル（Unicast/Multicast）をブロックするのだ
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return false, fmt.Errorf("制限されたネットワークへのアクセスを検知: %s", ip.String())
		}
	}

	return true, nil
}
