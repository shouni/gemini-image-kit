package generator

import (
	"context"
	"time"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
)

// AssetManager は File API や GCS とのやり取りを担当します。
type AssetManager interface {
	UploadFile(ctx context.Context, fileURI string) (string, error)
	DeleteFile(ctx context.Context, fileURI string) error
}

// ImageGenerator はビジネスロジック層が利用する統合窓口です。
type ImageGenerator interface {
	GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error)
	GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error)
}

// ImageExecutor は、画像生成リクエストを処理し、画像関連データを準備するためのメソッドを定義するインターフェースです。
type ImageExecutor interface {
	// ExecuteRequest は、指定されたパラメータで画像生成リクエストを実行し、結果を返します。
	ExecuteRequest(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*domain.ImageResponse, error)
	// PrepareImagePart は、指定された画像URLから後続処理で利用する画像パーツを作成します。
	PrepareImagePart(ctx context.Context, rawURL string) *genai.Part
}

// ImageCacher は、画像をキャッシュするためのインターフェースです。
type ImageCacher interface {
	// Get は、指定されたキーに紐づくアイテムを取得します。
	Get(key string) (any, bool)
	// Set は、指定されたキーと値、有効期限でアイテムを保存します。
	Set(key string, value any, d time.Duration)
}

// HTTPClient は、HTTPリクエストを実行し、URLからデータを取得するためのインターフェースです。
type HTTPClient interface {
	FetchBytes(ctx context.Context, url string) ([]byte, error)
}
