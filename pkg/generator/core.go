package generator

import (
	"context"
	"fmt"
	"net/http"
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
		}
	}

	// 2. SSRF対策（安全なURLかチェック）
	if !isSafeURL(url) {
		return nil
	}

	// 3. ダウンロード
	data, err := c.httpClient.FetchBytes(ctx, url)
	if err != nil {
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

// isSafeURL は簡単な SSRF 防止チェックを行うのだ。
func isSafeURL(urlStr string) bool {
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return false
	}
	// 実際の実装では net.LookupIP 等でプライベートIPを弾くロジックが入るのだ
	return true
}
