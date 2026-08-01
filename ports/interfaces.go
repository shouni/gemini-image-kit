package ports

import (
	"context"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
)

// Backend は、利用中のバックエンドサービス（Vertex AIなど）に関する状態や情報を提供します。
//
// gemini.BackendInspector の別名です。同じ 1 メソッドのインターフェースを 2 か所で定義すると、
// どちらを実装すればよいかが利用側から曖昧になります。
type Backend = gemini.BackendInspector

// ImageGenerator は、ビジネスロジック層が利用する統合窓口です。
type ImageGenerator interface {
	// GenerateSingleImage は、単一の参照画像と構成パラメータに基づいて画像を生成します。
	GenerateSingleImage(ctx context.Context, req SingleImageRequest) (*ImageResponse, error)
	// GenerateFusedImage は、複数の参照画像と構成パラメータに基づいて1枚の画像を生成します。
	GenerateFusedImage(ctx context.Context, req ImageFusionRequest) (*ImageResponse, error)
	Backend
}

// AssetManager は、File APIとのやり取りを担当します。
type AssetManager interface {
	// EnsureUploaded は指定された fileURI の画像を Gemini File API にアップロードし、
	// アップロード先の URI を返します。すでにアップロード済みならキャッシュの URI を返します。
	//
	// gemini.FileManager.UploadFile（io.Reader を受け取る低レベル API）とは
	// 役割が異なるため、名前を分けています。
	EnsureUploaded(ctx context.Context, fileURI string) (string, error)
	// DeleteFile は指定された URI を使用して Gemini File API からファイルを削除します。
	DeleteFile(ctx context.Context, fileURI string) error
}

// ImageExecutor は、画像生成リクエストを処理し、画像関連データを準備するためのメソッドを定義するインターフェースです。
type ImageExecutor interface {
	// ExecuteRequest は、指定されたパラメータで画像生成リクエストを実行し、結果を返します。
	ExecuteRequest(ctx context.Context, model string, prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions) (*ImageResponse, error)
	// PrepareImageAttachment は、指定された画像URLから後続処理で利用する添付を作成します。
	PrepareImageAttachment(ctx context.Context, rawURL string) (gemini.Attachment, error)
	Backend
}

// ImageCacher は、画像をキャッシュするためのインターフェースです。
//
// 参照画像の解決は複数の画像に対して並行に走るため、実装は同時アクセス安全である
// 必要があります（github.com/patrickmn/go-cache など、内部でロックする実装を使うか、
// 自前実装なら自分でロックしてください）。
type ImageCacher interface {
	// Get は、指定されたキーに紐づくアイテムを取得します。
	Get(key string) (any, bool)
	// Set は、指定されたキーと値、有効期限でアイテムを保存します。
	Set(key string, value any, d time.Duration)
	// Delete は、指定されたキーに紐づくアイテムを削除します。存在しないキーは無視します。
	Delete(key string)
}
