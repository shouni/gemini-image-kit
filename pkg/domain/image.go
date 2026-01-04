package domain

// ImageGenerationRequest は単一の画像生成要求です。
// Seed を *int32 にすることで Gemini SDK との直結を優先しています。
type ImageGenerationRequest struct {
	Prompt         string
	NegativePrompt string
	AspectRatio    string
	ReferenceURL   string
	Seed           *int64
}

// ImagePageRequest は漫画1ページの一括生成要求です。
type ImagePageRequest struct {
	Prompt         string
	NegativePrompt string
	AspectRatio    string
	ReferenceURLs  []string
	Seed           *int64
}

// ImageResponse は生成された画像データとそのメタデータです。
type ImageResponse struct {
	Data     []byte
	MimeType string
	UsedSeed int64 // 戻り値は情報欠落を防ぐため int64
}
