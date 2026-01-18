package generator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/shouni/gemini-image-kit/pkg/imgutil"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"github.com/shouni/go-remote-io/pkg/remoteio"
	"google.golang.org/genai"
)

const (
	// UseImageCompression は画像を送信前に圧縮するかどうかのフラグ
	UseImageCompression = true
	// ImageCompressionQuality は JPEG 圧縮の品質（1-100）
	ImageCompressionQuality = 75

	// キャッシュキー用のプレフィックス定数
	cacheKeyFileAPIURI  = "fileapi_uri:"
	cacheKeyFileAPIName = "fileapi_name:"
)

// ImageOutput は Core が解析した結果を保持する内部構造体
type ImageOutput struct {
	Data     []byte
	MimeType string
	UsedSeed int64
}

// HTTPClient は画像取得に必要な最小限のインターフェース
type HTTPClient interface {
	FetchBytes(ctx context.Context, url string) ([]byte, error)
}

// ImageCacher は画像のキャッシュを担当するインターフェース
type ImageCacher interface {
	Get(key string) (any, bool)
	Set(key string, value any, d time.Duration)
}

// GeminiImageCore は画像生成の基盤となるロジックを管理する
type GeminiImageCore struct {
	aiClient   gemini.GenerativeModel
	reader     remoteio.InputReader
	httpClient HTTPClient
	cache      ImageCacher
	expiration time.Duration
}

// NewGeminiImageCore は、画像操作を処理するための GeminiImageCore インスタンスを初期化して返す
func NewGeminiImageCore(aiClient gemini.GenerativeModel, reader remoteio.InputReader, client HTTPClient, cache ImageCacher, cacheTTL time.Duration) (*GeminiImageCore, error) {
	if aiClient == nil {
		return nil, fmt.Errorf("GenerativeModel は必須です")
	}
	if reader == nil {
		return nil, fmt.Errorf("InputReader は必須です")
	}
	if client == nil {
		return nil, fmt.Errorf("httpClient は必須です")
	}

	return &GeminiImageCore{
		aiClient:   aiClient,
		reader:     reader,
		httpClient: client,
		cache:      cache,
		expiration: cacheTTL,
	}, nil
}

// UploadFile は URI（GCS/HTTP）からデータを取得し、必要に応じて圧縮した上で Gemini File API へ転送します。
// 成功した場合、File API 上の URI を返します。
func (c *GeminiImageCore) UploadFile(ctx context.Context, fileURI string) (string, error) {
	// 1. 重複アップロード防止のためのキャッシュチェック
	cacheKeyURI := cacheKeyFileAPIURI + fileURI
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyURI); ok {
			if uri, ok := val.(string); ok {
				slog.DebugContext(ctx, "キャッシュされた File API URI を再利用します", "source", fileURI, "uri", uri)
				return uri, nil
			}
		}
	}

	// 2. 透過的なデータ取得 (GCS or HTTP)
	data, err := c.fetchImageData(ctx, fileURI)
	if err != nil {
		return "", fmt.Errorf("ソースデータの取得に失敗 (%s): %w", fileURI, err)
	}

	// 3. アップロード前の圧縮処理
	finalData := data
	if UseImageCompression {
		compressed, err := imgutil.CompressToJPEG(data, ImageCompressionQuality)
		if err != nil {
			slog.WarnContext(ctx, "アップロード前の圧縮に失敗したためオリジナルを使用します", "error", err)
		} else {
			finalData = compressed
			slog.DebugContext(ctx, "画像を圧縮してアップロード準備完了", "original", len(data), "compressed", len(finalData))
		}
	}

	// 4. ライブラリ (package gemini) の UploadFile を呼び出し
	mimeType := http.DetectContentType(finalData)
	displayName := filepath.Base(fileURI)
	uri, fileName, err := c.aiClient.UploadFile(ctx, finalData, mimeType, displayName)
	if err != nil {
		return "", fmt.Errorf("Gemini File API へのアップロードに失敗: %w", err)
	}

	// 5. 次回利用と削除のために情報をキャッシュ
	if c.cache != nil {
		c.cache.Set(cacheKeyURI, uri, c.expiration)
		c.cache.Set(cacheKeyFileAPIName+fileURI, fileName, c.expiration)
	}

	slog.InfoContext(ctx, "File API への転送に成功しました",
		"source", fileURI,
		"gemini_uri", uri,
	)

	return uri, nil
}

// DeleteFile は元の URI に紐付いた File API オブジェクトを削除します。
func (c *GeminiImageCore) DeleteFile(ctx context.Context, fileURI string) error {
	targetName := fileURI

	// キャッシュに管理名 (files/...) があればそちらを優先
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyFileAPIName + fileURI); ok {
			if name, ok := val.(string); ok {
				targetName = name
			}
		}
	}

	return c.aiClient.DeleteFile(ctx, targetName)
}

// prepareImagePart は URL から画像を準備し、最適な Part 形式（FileData または InlineData）に変換します。
func (c *GeminiImageCore) prepareImagePart(ctx context.Context, rawURL string) *genai.Part {
	// 1. 安全チェック (別ファイルで定義されていることを想定)
	if safe, err := IsSafeURL(rawURL); !safe {
		slog.WarnContext(ctx, "SSRFの可能性がある、または不正なURLをブロックしました", "url", rawURL, "error", err)
		return nil
	}

	// 2. File API キャッシュチェック (アップロード済みなら URI を参照)
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyFileAPIURI + rawURL); ok {
			if uri, ok := val.(string); ok {
				slog.DebugContext(ctx, "File API URI を参照します", "url", rawURL)
				return &genai.Part{
					FileData: &genai.FileData{
						FileURI: uri,
					},
				}
			}
		}
	}

	// 3. インラインデータ用のキャッシュチェック
	if c.cache != nil {
		if val, ok := c.cache.Get(rawURL); ok {
			if data, ok := val.([]byte); ok {
				slog.DebugContext(ctx, "キャッシュされた画像データを使用します", "url", rawURL)
				return c.toPart(data)
			}
		}
	}

	// 4. データ取得
	data, err := c.fetchImageData(ctx, rawURL)
	if err != nil {
		slog.ErrorContext(ctx, "画像の取得に失敗しました", "url", rawURL, "error", err)
		return nil
	}

	// 5. フラグに基づいた圧縮処理
	finalData := data
	if UseImageCompression {
		compressed, err := imgutil.CompressToJPEG(data, ImageCompressionQuality)
		if err != nil {
			slog.WarnContext(ctx, "圧縮に失敗したためオリジナルを使用します", "error", err)
		} else {
			finalData = compressed
		}
	}

	// 6. キャッシュ保存
	if c.cache != nil {
		c.cache.Set(rawURL, finalData, c.expiration)
	}

	return c.toPart(finalData)
}

// fetchImageData は URL スキームに基づいて適切な取得方法を選択します。
func (c *GeminiImageCore) fetchImageData(ctx context.Context, rawURL string) ([]byte, error) {
	if strings.HasPrefix(rawURL, "gs://") {
		return c.fetchFromGCS(ctx, rawURL)
	}

	return c.httpClient.FetchBytes(ctx, rawURL)
}

// fetchFromGCS は GCS からのデータ取得に特化したメソッドです。
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

// toPart はバイナリデータを MIME タイプ付きの InlineData Part に変換します。
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

// parseToResponse は Gemini のレスポンスから画像データを抽出します。
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
