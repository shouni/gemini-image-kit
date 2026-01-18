package domain

// ImageGenerationRequest は単一の画像生成要求です。
type ImageGenerationRequest struct {
	Prompt         string
	SystemPrompt   string
	NegativePrompt string
	AspectRatio    string
	ReferenceURL   string // 元の参照先 (GCS, HTTP等)
	FileAPIURI     string // Gemini File API 上の URI (https://...) ★追加
	Seed           *int64
}

// ImagePageRequest は漫画1ページの一括生成要求です。
type ImagePageRequest struct {
	Prompt         string
	SystemPrompt   string
	NegativePrompt string
	AspectRatio    string
	ReferenceURLs  []string // 元の参照先リスト
	FileAPIURIs    []string // Gemini File API 上の URI リスト ★追加
	Seed           *int64
}

// ImageResponse は生成された画像データとそのメタデータです。
type ImageResponse struct {
	Data     []byte
	MimeType string
	UsedSeed int64
}
