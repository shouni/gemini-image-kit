package domain

import (
	"context"
	"time"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
)

// ImageGenerator は、ビジネスロジック層が利用する統合窓口です。
type ImageGenerator interface {
	// GetImageExecutor は、画像生成リクエストの処理を担当する基盤となる ImageExecutor インスタンスを返します。
	GetImageExecutor() ImageExecutor
	// GenerateMangaPanel は、提供されたプロンプトと構成パラメータに基づいて、単一のマンガパネル画像を生成します。
	GenerateMangaPanel(ctx context.Context, req ImageGenerationRequest) (*ImageResponse, error)
	// GenerateMangaPage は、提供されたプロンプトと構成パラメータに基づいてマンガページのイメージを生成します。
	GenerateMangaPage(ctx context.Context, req ImagePageRequest) (*ImageResponse, error)
}

// AssetManager は、File APIとのやり取りを担当します。
type AssetManager interface {
	// UploadFile は指定された fileURI の画像を Gemini File API にアップロードし、アップロード先の URI を返します。
	UploadFile(ctx context.Context, fileURI string) (string, error)
	// DeleteFile は指定された URI を使用して Gemini File API からファイルを削除します。
	DeleteFile(ctx context.Context, fileURI string) error
}

// ImageExecutor は、画像生成リクエストを処理し、画像関連データを準備するためのメソッドを定義するインターフェースです。
type ImageExecutor interface {
	// GetGenerativeModel は、基盤となる GenerativeModel インスタンスを返します。
	GetGenerativeModel() gemini.GenerativeModel
	// ExecuteRequest は、指定されたパラメータで画像生成リクエストを実行し、結果を返します。
	ExecuteRequest(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*ImageResponse, error)
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
