// Package ports は、gemini-image-kit の各コンポーネントが依存する
// インターフェース（ポート）と、画像生成・編集に関する共通データ型を定義します。
package ports

import (
	"github.com/shouni/go-gemini-client/gemini"
	"google.golang.org/genai"
)

// ImageURI は画像の参照先情報を保持します。
type ImageURI struct {
	ReferenceURL string // 元の参照先 (GCS, HTTP等)
	FileAPIURI   string // Gemini File API 上の URI (https://...)
}

// IsEmpty は画像参照先が設定されていないかを返します。
func (uri ImageURI) IsEmpty() bool {
	return uri.ReferenceURL == "" && uri.FileAPIURI == ""
}

// GenerationOptions は画像生成時の共通設定パラメータを保持します。
type GenerationOptions struct {
	Model          string
	Prompt         string
	SystemPrompt   string
	NegativePrompt string
	AspectRatio    string
	ImageSize      string
	Seed           *int64
	// PersonGeneration は人物生成の許可ポリシーです。
	// Vertex AI バックエンドでのみ適用され、未指定時は PersonGenerationAllowAll になります。
	// Gemini API バックエンドではこのフィールドを設定すると API エラーになるため、常に無視されます。
	PersonGeneration gemini.PersonGeneration

	// 以下は gemini.GenerateOptions へそのまま渡される生成パラメータです。
	// ゼロ値が意味を持つ項目はポインタで、nil は「SDK のデフォルトに委ねる」を意味します。
	// 設定には gemini.Ptr ヘルパーが使えます。
	//
	// これらは Gemini API 上で非推奨ではありませんが、画像生成モデルが
	// どこまで解釈するかはモデル依存です（テキスト生成向けのパラメータのため、
	// 無視される場合があります）。

	// Temperature は出力のランダム性です。nil で SDK デフォルト。
	Temperature *float32
	// TopP は核サンプリングの閾値です。nil で SDK デフォルト。
	TopP *float32
	// MaxOutputTokens は生成する最大トークン数です。0 で SDK デフォルト。
	MaxOutputTokens int32
	// ThinkingBudget は思考トークンの上限です。
	// gemini.Ptr[int32](0) で思考を無効化し、レイテンシとコストを抑えられます。
	// 有効範囲はモデル依存のため、モデルを跨ぐ場合は ThinkingLevel を推奨します。
	ThinkingBudget *int32
	// ThinkingLevel は思考量の段階指定です（MINIMAL / LOW / MEDIUM / HIGH）。
	// ThinkingBudget と併用した場合はこちらが優先されます。
	ThinkingLevel genai.ThinkingLevel
}

// SingleImageRequest は単一の参照画像を使う画像生成要求です。
type SingleImageRequest struct {
	GenerationOptions
	Image ImageURI
}

// ImageFusionRequest は複数の参照画像を統合して1枚の画像を生成する要求です。
type ImageFusionRequest struct {
	GenerationOptions
	Images []ImageURI
}

// ImageResponse は生成された画像データとそのメタデータです。
type ImageResponse struct {
	Data     []byte
	MimeType string
	UsedSeed int64
}
