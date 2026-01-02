package domain

// Character は漫画に登場するキャラクターの定義を保持します。
type Character struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	VisualCues   []string `json:"visual_cues"`   // 生成プロンプトに注入する外見上の特徴
	ReferenceURL string   `json:"reference_url"` // 一貫性保持のための参照画像URL
	Seed         int64    `json:"seed"`          // DB保存等のために広い型を維持
}

// MangaResponse はAIから出力される漫画全体の構成案を保持します。
type MangaResponse struct {
	Title string      `json:"title"`
	Pages []MangaPage `json:"pages"`
}

// MangaPage は漫画の1ページまたは1パネルの構成、セリフ、話者情報を保持します。
type MangaPage struct {
	Page         int    `json:"page"`
	VisualAnchor string `json:"visual_anchor"`
	Dialogue     string `json:"dialogue"`
	SpeakerID    string `json:"speaker_id"`
	ReferenceURL string `json:"reference_url"`
}

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

// Panel は最終的な画像とテキストの統合成果物です。
type Panel struct {
	PageNumber int
	PanelIndex int
	Prompt     string
	Dialogue   string
	Character  *Character
	ImageBytes []byte
}
