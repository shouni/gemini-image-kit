package generator

import (
	"context"

	"github.com/shouni/gemini-image-kit/pkg/domain"
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
