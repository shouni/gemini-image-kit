package domain

// ImageGenerationRequest は単一の画像生成要求です。
// Seed を *int32 にすることで Gemini SDK との直結を優先しています。
type ImageGenerationRequest struct {
	Prompt         string
	NegativePrompt string
	AspectRatio    string
	Seed           *int32 // nil でランダム、値指定で固定。Gemini SDK 互換
	ReferenceURL   string
}

// ImagePageRequest は漫画1ページの一括生成要求です。
type ImagePageRequest struct {
	Prompt         string
	NegativePrompt string
	ReferenceURLs  []string
	AspectRatio    string
	Seed           *int32 // ImageGenerationRequest と型を統一
}

// ImageResponse は生成された画像データとそのメタデータです。
type ImageResponse struct {
	Data     []byte
	MimeType string
	UsedSeed int64 // 戻り値は情報欠落を防ぐため int64
}
