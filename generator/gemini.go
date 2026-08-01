package generator

import (
	"context"

	"github.com/shouni/gemini-image-kit/ports"
)

const negativePromptSeparator = "\n\n[Negative Prompt]\n"

// GeminiGenerator は高レベルな画像生成ロジックを担当します。
type GeminiGenerator struct {
	core     ports.ImageExecutor
	autoSeed bool
}

// Option は GeminiGenerator の任意設定です。
type Option func(*GeminiGenerator)

// WithAutoSeed は、リクエストに Seed が指定されていない場合に、生成側でシードを
// 決めてから送信するようにします。
//
// これを使わない場合、Seed 未指定の生成は API 側がランダムなシードを選び、その値は
// レスポンスに含まれません。ImageResponse.UsedSeed は 0 のままになるため、それを
// 「使われたシード」として記録すると、同条件での再生成ができない（あるいは 0 という
// 別のシードで再生成してしまう）ことになります。
//
// 有効にすると、シード未指定の生成でも UsedSeed が実際に送ったシードを指すため、
// 記録しておけば後から同じ結果を再現できます。生成結果のランダム性は変わりません
// （シードを選ぶのが API 側か生成側かの違いです）。
func WithAutoSeed() Option {
	return func(g *GeminiGenerator) {
		g.autoSeed = true
	}
}

// NewGeminiGenerator は新しい GeminiGenerator を作成します。
func NewGeminiGenerator(core ports.ImageExecutor, opts ...Option) (*GeminiGenerator, error) {
	if core == nil {
		return nil, ErrExecutorRequired
	}

	g := &GeminiGenerator{
		core: core,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(g)
		}
	}
	return g, nil
}

// IsVertexAI は、Vertex AI バックエンドを使用しているかを確認します。
func (g *GeminiGenerator) IsVertexAI() bool {
	return g.core.IsVertexAI()
}

// GenerateSingleImage は単一の参照画像を使って画像を生成します。
func (g *GeminiGenerator) GenerateSingleImage(ctx context.Context, req ports.SingleImageRequest) (*ports.ImageResponse, error) {
	return g.generate(
		ctx,
		req.GenerationOptions,
		[]ports.ImageURI{req.Image},
	)
}

// GenerateFusedImage は複数の参照画像を統合して1枚の画像を生成します。
func (g *GeminiGenerator) GenerateFusedImage(ctx context.Context, req ports.ImageFusionRequest) (*ports.ImageResponse, error) {
	return g.generate(
		ctx,
		req.GenerationOptions,
		req.Images,
	)
}
