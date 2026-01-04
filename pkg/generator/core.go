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
// 汎用性を保つため、値の型には any (interface{}) を使用するのだ。
type ImageCacher interface {
	Get(key string) (any, bool)
	Set(key string, value any, d time.Duration)
}

// GeminiImageCore は画像生成の基盤となるロジックを管理するのだ。
type GeminiImageCore struct {
	httpClient HTTPClient
	cache      ImageCacher
	expiration time.Duration
}

// NewGeminiImageCore は、画像操作を処理するための GeminiImageCore インスタンスを初期化して返すのだ。
// HTTPClient は必須なのだ。ImageCacher は nil でも動作する（キャッシュしないだけ）設計なのだよ。
func NewGeminiImageCore(client HTTPClient, cache ImageCacher, cacheTTL time.Duration) (*GeminiImageCore, error) {
	if client == nil {
		return nil, fmt.Errorf("httpClient (generator.HTTPClient) は必須なのだ")
	}

	return &GeminiImageCore{
		httpClient: client,
		cache:      cache,
		expiration: cacheTTL,
	}, nil
}

// prepareImagePart は URL から画像を準備し、Gemini 用の Part に変換するのだ。
func (c *GeminiImageCore) prepareImagePart(ctx context.Context, url string) *genai.Part {
	// 1. キャッシュチェック（cache が設定されている場合のみ）
	if c.cache != nil {
		if val, ok := c.cache.Get(url); ok {
			// キャッシュから取り出した値が []byte であることを確認するのだ
			if data, ok := val.([]byte); ok {
				return c.toPart(data)
			}
			slog.WarnContext(ctx, "キャッシュに不正な型のデータが含まれています", "url", url)
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

	// 4. キャッシュ保存（cache が設定されている場合のみ）
	if c.cache != nil {
		c.cache.Set(url, data, c.expiration)
	}

	return c.toPart(data)
}

// toPart はバイナリデータを MIME タイプ付きの Part に変換するのだ。
func (c *GeminiImageCore) toPart(data []byte) *genai.Part {
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

// parseToResponse は Gemini のレスポンスから画像データを抽出するのだ。
func (c *GeminiImageCore) parseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error) {
	if resp == nil || resp.RawResponse == nil {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	raw := resp.RawResponse
	if len(raw.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	candidate := raw.Candidates[0]
	// FinishReasonStop または Unspecified 以外はエラーとして扱うのだ
	if candidate.FinishReason != genai.FinishReasonStop && candidate.FinishReason != genai.FinishReasonUnspecified {
		return nil, fmt.Errorf("generation failed with FinishReason: %s", candidate.FinishReason)
	}

	if candidate.Content == nil {
		return nil, fmt.Errorf("no content found in candidate")
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
		return false, fmt.Errorf("ホスト '%s' の名前解決に失敗しました: %w", parsedURL.Hostname(), err)
	}

	if len(ips) == 0 {
		return false, fmt.Errorf("IPアドレスが見つかりません")
	}

	for _, ip := range ips {
		// プライベート、ループバック、リンクローカルをブロックして安全を確保するのだ
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return false, fmt.Errorf("制限されたネットワークへのアクセスを検知: %s", ip.String())
		}
	}

	return true, nil
}
