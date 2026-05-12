package ports

// ImageURI は画像の参照先情報を保持します。
type ImageURI struct {
	ReferenceURL string // 元の参照先 (GCS, HTTP等)
	FileAPIURI   string // Gemini File API 上の URI (https://...)
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
