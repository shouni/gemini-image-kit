// Package ports は、gemini-image-kit の各コンポーネントが依存する
// インターフェース（ポート）と、画像生成・編集に関する共通データ型を定義します。
package ports

import "github.com/shouni/go-gemini-client/gemini"

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
