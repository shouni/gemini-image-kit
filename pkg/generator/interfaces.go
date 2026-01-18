package generator

import (
	"context"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
)

// ImageGenerator はビジネスロジック層が利用する統合窓口です。
type ImageGenerator interface {
	GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error)
	GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error)
}

// AssetManager は File API や GCS とのやり取りを担当します。
type AssetManager interface {
	UploadFile(ctx context.Context, fileURI string) (string, error)
	DeleteFile(ctx context.Context, fileURI string) error
}

type ImageGeneratorCore interface {
	executeRequest(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*domain.ImageResponse, error)
	prepareImagePart(ctx context.Context, rawURL string) *genai.Part
}
