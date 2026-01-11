package generator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/shouni/gemini-image-kit/pkg/imgutil"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"github.com/shouni/go-remote-io/pkg/remoteio"
	"google.golang.org/genai"
)

const (
	// UseImageCompression は画像を送信前に圧縮するかどうかのフラグなのだ
	UseImageCompression = true
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
	reader     remoteio.InputReader
	httpClient HTTPClient
	cache      ImageCacher
	expiration time.Duration
}

// NewGeminiImageCore は、画像操作を処理するための GeminiImageCore インスタンスを初期化して返すのだ。
func NewGeminiImageCore(reader remoteio.InputReader, client HTTPClient, cache ImageCacher, cacheTTL time.Duration) (*GeminiImageCore, error) {
	if reader == nil {
		return nil, fmt.Errorf("InputReader は必須です")
	}
	if client == nil {
		return nil, fmt.Errorf("httpClient は必須です")
	}

	return &GeminiImageCore{
		reader:     reader,
		httpClient: client,
		cache:      cache,
		expiration: cacheTTL,
	}, nil
}

// PrepareImagePart は URL から画像を準備し、インラインデータ形式の Part に変換するなのだ。
func (c *GeminiImageCore) prepareImagePart(ctx context.Context, rawURL string) *genai.Part {
	// 1. 安全チェック
	if safe, err := IsSafeURL(rawURL); !safe {
		slog.WarnContext(ctx, "SSRFの可能性がある、または不正なURLをブロックしました", "url", rawURL, "error", err)
		return nil
	}

	// 2. キャッシュチェック
	if c.cache != nil {
		if val, ok := c.cache.Get(rawURL); ok {
			if data, ok := val.([]byte); ok {
				slog.DebugContext(ctx, "キャッシュされた画像データを使用するのだ", "url", rawURL)
				return c.toPart(data)
			}
		}
	}

	// 3. データ取得
	data, err := c.fetchImageData(ctx, rawURL)
	if err != nil {
		slog.ErrorContext(ctx, "画像の取得に失敗したのだ", "url", rawURL, "error", err)
		return nil
	}

	// 4. フラグに基づいた圧縮処理
	finalData := data
	if UseImageCompression {
		compressed, err := imgutil.CompressToJPEG(data, ImageCompressionQuality)
		if err != nil {
			slog.WarnContext(ctx, "圧縮に失敗したためオリジナルを使用するのだ", "error", err)
		} else {
			finalData = compressed
			slog.DebugContext(ctx, "画像を圧縮したのだ", "original_size", len(data), "new_size", len(finalData))
		}
	}

	// 5. キャッシュ保存
	if c.cache != nil {
		c.cache.Set(rawURL, finalData, c.expiration)
	}

	return c.toPart(finalData)
}

// fetchImageData は URL スキームに基づいて適切な取得方法を選択するのだ。
func (c *GeminiImageCore) fetchImageData(ctx context.Context, rawURL string) ([]byte, error) {
	if strings.HasPrefix(rawURL, "gs://") {
		return c.fetchFromGCS(ctx, rawURL)
	}

	return c.httpClient.FetchBytes(ctx, rawURL)
}

// fetchFromGCS は GCS からのデータ取得に特化したメソッドなのだ。
func (c *GeminiImageCore) fetchFromGCS(ctx context.Context, url string) ([]byte, error) {
	slog.DebugContext(ctx, "GCSから画像を取得", "url", url)
	rc, err := c.reader.Open(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("GCSファイルのオープンに失敗: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("GCSファイルの読み取りに失敗: %w", err)
	}
	return data, nil
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
