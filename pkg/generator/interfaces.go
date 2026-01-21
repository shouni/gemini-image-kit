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

// ImageExecutor は、画像生成リクエストを処理し、画像関連データを準備するためのメソッドを定義します。
type ImageExecutor interface {
	// ExecuteRequest は、指定されたパラメータを使用して画像を生成するリクエストを送信し、結果の画像データを返します。
	ExecuteRequest(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*domain.ImageResponse, error)
	// PrepareImagePart は、指定された生の画像 URL から、さらに処理するための画像パーツ オブジェクトを作成します。
	PrepareImagePart(ctx context.Context, rawURL string) *genai.Part
}

// ImageCacher は、キーと値のペアを使用してアイテムを取得および保存するメソッドを備えた、画像をキャッシュするためのインターフェースを定義します。
type ImageCacher interface {
	// Get メソッドは、指定されたキーに関連付けられたアイテムを取得し、その値とその存在を示すブール値を返します。
	Get(key string) (any, bool)
	// Set メソッドは、指定されたキー、値、および有効期限を持つアイテムを保存します。
	Set(key string, value any, d time.Duration)
}

// HTTPClient HTTP リクエストを実行し、指定された URL から生のバイト データを取得するためのインターフェイスを定義します。
type HTTPClient interface {
	FetchBytes(ctx context.Context, url string) ([]byte, error)
}
