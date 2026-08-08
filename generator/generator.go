package generator

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"golang.org/x/time/rate"

	"github.com/shouni/gemini-image-kit/ports"
)

// GeminiGenerator がポートを満たすことをコンパイル時に保証します。
var _ ports.BatchImageGenerator = (*GeminiGenerator)(nil)

// GeminiGenerator は高レベルな画像生成ロジックを担当します。
//
// レート制限・並列度・リクエストタイムアウトを内蔵しており、複数画像の生成で
// 利用側が errgroup + rate.Limiter を自前で組む必要はありません（以前は 3 つの
// 下流リポジトリがそれぞれ同じガードを再実装していました）。
type GeminiGenerator struct {
	core           imageExecutor
	autoSeed       bool
	limiter        *rate.Limiter
	maxConcurrency int
	requestTimeout time.Duration
}

// imageExecutor は GeminiGenerator が依存する下位層（GeminiImageCore）の面です。
// パッケージ外に公開しないのは、この分割が実装の都合であって利用側の契約では
// ないためです（外部の利用者は ports.ImageGenerator に依存します）。
type imageExecutor interface {
	executeRequest(ctx context.Context, model string, prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions) (*ports.ImageResponse, error)
	resolveReference(ctx context.Context, uri ports.ImageURI) (gemini.Attachment, error)
	IsVertexAI() bool
}

// Option は GeminiGenerator の任意設定です。
type Option func(*GeminiGenerator)

// WithoutAutoSeed は、シード未指定のリクエストへの自動採番を無効にします。
//
// 既定では、Seed が未指定の生成にはランダムなシードを採番してから送信します。
// API 側にシード選択を委ねるとその値はレスポンスに含まれず、ImageResponse.UsedSeed
// が 0 のまま記録されて同条件での再生成ができなくなるためです（0 は有効なシード
// なので、「未記録」と区別も付きません）。自動採番しても生成結果のランダム性は
// 変わりません — シードを選ぶのが API 側か生成側かの違いです。
//
// このオプションは、シード管理を完全に呼び出し側で行う場合にのみ使ってください。
func WithoutAutoSeed() Option {
	return func(g *GeminiGenerator) {
		g.autoSeed = false
	}
}

// WithRateLimit は、生成リクエストの発射間隔と許容バーストを設定します。
// interval が 0 以下の場合はレート制限を行いません（既定）。
func WithRateLimit(interval time.Duration, burst int) Option {
	return func(g *GeminiGenerator) {
		if interval <= 0 {
			g.limiter = nil
			return
		}
		if burst < 1 {
			burst = 1
		}
		g.limiter = rate.NewLimiter(rate.Every(interval), burst)
	}
}

// WithMaxConcurrency は、GenerateBatch の最大並列数を設定します。
// 1 以下（既定は 1）なら逐次実行です。
func WithMaxConcurrency(n int) Option {
	return func(g *GeminiGenerator) {
		if n > 0 {
			g.maxConcurrency = n
		}
	}
}

// WithRequestTimeout は、生成リクエスト 1 回あたりの上限時間を設定します。
// 0 以下（既定）は無制限で、呼び出し側の context にのみ従います。
// 画像生成は分単位で掛かることがあるため、余裕を持った値にしてください。
func WithRequestTimeout(d time.Duration) Option {
	return func(g *GeminiGenerator) {
		g.requestTimeout = d
	}
}

// NewGeminiGenerator は新しい GeminiGenerator を作成します。
func NewGeminiGenerator(core *GeminiImageCore, opts ...Option) (*GeminiGenerator, error) {
	if core == nil {
		return nil, ErrExecutorRequired
	}

	g := &GeminiGenerator{
		core:           core,
		autoSeed:       true,
		maxConcurrency: 1,
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

// Generate は、参照画像（0〜複数）と構成パラメータに基づいて 1 枚の画像を生成します。
// WithRateLimit / WithRequestTimeout が設定されていれば、この呼び出しにも適用されます。
func (g *GeminiGenerator) Generate(ctx context.Context, req ports.ImageRequest) (*ports.ImageResponse, error) {
	if g.limiter != nil {
		if err := g.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	ctx, cancel := g.requestContext(ctx)
	defer cancel()

	return g.generate(ctx, req.GenerationOptions, req.Images)
}

// GenerateBatch は複数のリクエストを設定された並列度・レート制限の下で実行します。
//
// 結果は入力と同じ順序で返します。一部の失敗で残りを打ち切らず、成功した結果も
// 破棄しません — 画像 1 枚ごとに生成コストが掛かっており、支払い済みの成果物を
// 1 件の失敗で捨てないためです。失敗した位置の要素は nil になり、エラーは
// errors.Join で集約されます（呼び出し側は結果とエラーの両方を見てください）。
func (g *GeminiGenerator) GenerateBatch(ctx context.Context, reqs []ports.ImageRequest) ([]*ports.ImageResponse, error) {
	results := make([]*ports.ImageResponse, len(reqs))
	errs := make([]error, len(reqs))

	concurrency := g.maxConcurrency
	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	for i := range reqs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 呼び出し側の context が終了していたら新しい生成は始めない
			//（進行中のものはそれぞれの ctx で打ち切られる）。
			if err := ctx.Err(); err != nil {
				errs[i] = err
				return
			}
			results[i], errs[i] = g.Generate(ctx, reqs[i])
		}()
	}
	wg.Wait()

	return results, errors.Join(errs...)
}

// requestContext は WithRequestTimeout を適用した context を返します。
func (g *GeminiGenerator) requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if g.requestTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, g.requestTimeout)
}

// generate は画像生成のコアロジックです。
func (g *GeminiGenerator) generate(ctx context.Context, req ports.GenerationOptions, uris []ports.ImageURI) (*ports.ImageResponse, error) {
	if req.Model == "" {
		return nil, ErrModelRequired
	}
	finalPrompt := buildFinalPrompt(req.Prompt, req.NegativePrompt)
	if finalPrompt == "" {
		return nil, ErrEmptyPrompt
	}
	// シードを生成側で決めるのは送信前の1回だけ。executeRequest は opts.Seed を
	// そのまま UsedSeed として返すため、ここで埋めた値が呼び出し側に届く。
	if g.autoSeed && req.Seed == nil {
		req.Seed = newSeed()
	}

	// 1. 画像アセット（素材）を収集
	attachments, err := g.collectImageAttachments(ctx, uris)
	if err != nil {
		return nil, err
	}

	// 2. バックエンド既定（安全設定・人物生成）を補ったオプション構築
	opts := g.toOptions(req)
	return g.core.executeRequest(ctx, req.Model, finalPrompt, attachments, opts)
}
