package ports

import (
	"context"
	"time"
)

// ImageGenerator は、ビジネスロジック層が利用する画像生成の窓口です。
//
// 意図的に 1 メソッドです。以前はバックエンド判定（IsVertexAI）を埋め込んでいた
// ため、テスト用フェイクを書く利用側が全員それも実装させられ、結局どの下流も
// 自前の 1 メソッドインターフェースを宣言していました。バックエンド判定が必要な
// 場合は gemini.BackendInspector に別途依存してください。
type ImageGenerator interface {
	// Generate は、参照画像（0〜複数）と構成パラメータに基づいて 1 枚の画像を生成します。
	Generate(ctx context.Context, req ImageRequest) (*ImageResponse, error)
}

// BatchImageGenerator は、複数リクエストの一括生成を行う窓口です。
//
// 実装（generator.Generator）はレート制限・並列度・リクエストタイムアウトを
// 内蔵しているため、利用側で errgroup + rate.Limiter を組む必要はありません。
type BatchImageGenerator interface {
	ImageGenerator
	// GenerateBatch は複数のリクエストを設定された並列度・レート制限の下で実行し、
	// 入力と同じ順序で結果を返します。一部が失敗しても成功した結果は破棄されず、
	// 失敗した位置の要素だけが nil になります（エラーは requests[i] の添字を付けて
	// errors.Join で集約されます）。
	GenerateBatch(ctx context.Context, reqs []ImageRequest) ([]*ImageResponse, error)
}

// ImageCacher は、アップロード済み参照のキャッシュを提供するインターフェースです。
//
// 参照画像の解決は複数の画像に対して並行に走るため、実装は同時アクセス安全である
// 必要があります（内部でロックする実装を使うか、自前実装なら自分でロックしてください）。
type ImageCacher interface {
	// Get は、指定されたキーに紐づくアイテムを取得します。
	Get(key string) (any, bool)
	// Set は、指定されたキーと値、有効期限でアイテムを保存します。
	Set(key string, value any, d time.Duration)
}
