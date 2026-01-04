package generator

import (
	"context"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// ImageGenerator は単一のパネル画像を生成するためのインターフェースなのだ。
type ImageGenerator interface {
	GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error)
}

// MangaPageGenerator は複数の参照画像を用いてマンガのページを生成するためのインターフェースなのだ。
type MangaPageGenerator interface {
	GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error)
}

// ImageGeneratorCore は画像データの取得やパースなど、AI通信の前後処理を担うインターフェースなのだ。
type ImageGeneratorCore interface {
	PrepareImagePart(ctx context.Context, url string) *genai.Part
	ToPart(data []byte) *genai.Part
	ParseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error)
}
