package generator

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

const (
	// UseImageCompression は画像を送信前に圧縮するかどうかのフラグなのだ
	UseImageCompression = false
	// ImageCompressionQuality は JPEG 圧縮の品質（1-100）なのだ
	ImageCompressionQuality = 75
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

// GeminiImageCore は画像生成の基盤となるロジックを管理するのだ。
type GeminiImageCore struct {
	httpClient HTTPClient
	cache      ImageCacher
	expiration time.Duration
}

// NewGeminiImageCore は、画像操作を処理するための GeminiImageCore インスタンスを初期化して返すのだ。
func NewGeminiImageCore(client HTTPClient, cache ImageCacher, cacheTTL time.Duration) (*GeminiImageCore, error) {
	if client == nil {
		return nil, fmt.Errorf("httpClient は必須なのだ")
	}

	return &GeminiImageCore{
		httpClient: client,
		cache:      cache,
		expiration: cacheTTL,
	}, nil
}

// PrepareImagePart は URL から画像を準備し、インラインデータ形式の Part に変換する
func (c *GeminiImageCore) prepareImagePart(ctx context.Context, rawURL string) *genai.Part {
	// 1. キャッシュチェック（[]byte をキャッシュから探すのだ）
	if c.cache != nil {
		if val, ok := c.cache.Get(rawURL); ok {
			if data, ok := val.([]byte); ok {
				slog.DebugContext(ctx, "キャッシュされた画像データを使用するのだ", "url", rawURL)
				return c.toPart(data)
			}
		}
	}

	// 2. SSRF対策
	if safe, err := isSafeURL(rawURL); !safe {
		slog.WarnContext(ctx, "SSRFの可能性がある、または不正なURLをブロックしました", "url", rawURL, "error", err)
		return nil
	}

	// 3. ダウンロード
	data, err := c.httpClient.FetchBytes(ctx, rawURL)
	if err != nil {
		slog.ErrorContext(ctx, "画像の取得に失敗したのだ", "url", rawURL, "error", err)
		return nil
	}

	// 4. フラグに基づいた圧縮処理
	finalData := data
	if UseImageCompression {
		compressed, err := c.compressImage(data, ImageCompressionQuality)
		if err != nil {
			slog.WarnContext(ctx, "圧縮に失敗したためオリジナルを使用するのだ", "error", err)
		} else {
			finalData = compressed
			slog.DebugContext(ctx, "画像を圧縮したのだ", "original_size", len(data), "new_size", len(finalData))
		}
	}
	// 5. キャッシュ保存（圧縮済みのデータを保存するのだ）
	if c.cache != nil {
		c.cache.Set(rawURL, finalData, c.expiration)
	}

	return c.toPart(finalData)
}

// toPart はバイナリデータを MIME タイプ付きの InlineData Part に変換するのだ。
func (c *GeminiImageCore) toPart(data []byte) *genai.Part {
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

	for _, ip := range ips {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return false, fmt.Errorf("制限されたネットワークへのアクセスを検知: %s", ip.String())
		}
	}

	return true, nil
}

// compressImage は画像を JPEG 形式に圧縮してバイト列を返す。
func (c *GeminiImageCore) compressImage(data []byte, quality int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	opt := jpeg.Options{Quality: quality}
	if err := jpeg.Encode(buf, img, &opt); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
