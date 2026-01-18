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

//// ImageGeneratorCore は AI モデルとの具体的な通信とデータ変換を担当します。
//type ImageGeneratorCore interface {
//	ExecuteRequest(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*domain.ImageResponse, error)
//	PrepareImagePart(ctx context.Context, url string) (*genai.Part, error)
//}
