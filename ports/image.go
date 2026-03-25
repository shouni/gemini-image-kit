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

// ImagePanelRequest は単一の画像生成要求です。
type ImagePanelRequest struct {
	GenerationOptions
	Image ImageURI
}

// ImagePageRequest は漫画1ページの一括生成要求です。
type ImagePageRequest struct {
	GenerationOptions
	Images []ImageURI
}

// ImageResponse は生成された画像データとそのメタデータです。
type ImageResponse struct {
	Data     []byte
	MimeType string
	UsedSeed int64
}
