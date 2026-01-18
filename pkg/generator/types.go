package generator

import (
	"context"

	"github.com/shouni/gemini-image-kit/pkg/domain"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
)

// ImageGenerator は、単一パネル生成とページ一括生成の両方を担当する統合インターフェースなのだ！
// 利用側はこのインターフェース一つを依存関係として受け取れば、すべての生成機能にアクセスできるのだよ。
type ImageGenerator interface {
	// GenerateMangaPanel は単一の参照画像（または参照なし）からパネルを生成するのだ。
	GenerateMangaPanel(ctx context.Context, req domain.ImageGenerationRequest) (*domain.ImageResponse, error)
	// GenerateMangaPage は複数の参照画像を用いてマンガの1ページ分を生成するのだ。
	GenerateMangaPage(ctx context.Context, req domain.ImagePageRequest) (*domain.ImageResponse, error)
}

// ImageGeneratorCore は画像データの取得やパースなど、AI通信の前後処理を担うインターフェースなのだ。
type ImageGeneratorCore interface {
	// UploadFile は指定されたURIからデータを取得し、File APIへアップロードして、
	// 生成された File API 上の URI (https://...) を返します。
	UploadFile(ctx context.Context, fileURI string) (string, error)
	// DeleteFile は指定されたパス（URI）のファイルを削除します。
	DeleteFile(ctx context.Context, fileURI string) error

	prepareImagePart(ctx context.Context, url string) *genai.Part
	toPart(data []byte) *genai.Part
	parseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error)
}
