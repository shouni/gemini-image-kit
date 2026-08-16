// Package generator は、Gemini API / Vertex AI 上の画像モデルによる画像生成を実行します。
//
// 参照画像の送り方は ports.ReferenceResolver として外から注入します。gs:// をそのまま
// 参照するのか、File API へ上げて使い回すのか、取得してインラインで送るのかは
// バックエンドと運用で変わるためです。依存（取得・キャッシュ）は選んだ resolver だけが
// 要求するので、gs:// しか使わない構成では何も渡さずに済みます。
package generator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"golang.org/x/time/rate"

	"github.com/shouni/gemini-image-kit/ports"
)

// Generator がポートを満たすことをコンパイル時に保証します。
var _ ports.BatchImageGenerator = (*Generator)(nil)

// Generator は画像生成の実装です。
//
// レート制限・並列度・リクエストタイムアウトを内蔵しており、利用側が errgroup +
// rate.Limiter を自前で組む必要はありません（かつては下流の 3 リポジトリが
// それぞれ同じガードを再実装していました）。
type Generator struct {
	aiClient       gemini.Generator
	resolver       ports.ReferenceResolver
	isVertexAI     bool
	autoSeed       bool
	limiter        *rate.Limiter
	maxConcurrency int
	requestTimeout time.Duration
}

// Option は Generator の任意設定です。
type Option func(*Generator)

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
	return func(g *Generator) {
		g.autoSeed = false
	}
}

// WithRateLimit は、生成リクエストの発射間隔と許容バーストを設定します。
// interval が 0 以下の場合はレート制限を行いません（既定）。
//
// スループットは WithMaxConcurrency によらず 1/interval で頭打ちになります。
// 発射間隔と並列度の両方を大きくする設定は矛盾しています。
func WithRateLimit(interval time.Duration, burst int) Option {
	return func(g *Generator) {
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
	return func(g *Generator) {
		if n > 0 {
			g.maxConcurrency = n
		}
	}
}

// WithRequestTimeout は、生成リクエスト 1 回あたりの上限時間を設定します。
// 0 以下（既定）は無制限で、呼び出し側の context にのみ従います。
// 画像生成は分単位で掛かることがあるため、余裕を持った値にしてください。
func WithRequestTimeout(d time.Duration) Option {
	return func(g *Generator) {
		g.requestTimeout = d
	}
}

// New は Generator を作成します。
//
// client は gemini.Generator（GenerateWithAttachments の 1 メソッド）で足ります。
// gemini.Model を要求しないのは、File API 管理（UploadFile / DeleteFile）が
// FileAPIResolver 側の関心になり、それを使わない構成のテスト用フェイクに実装させて
// しまうためです。
//
// resolver は必須です。既定を用意すると、参照の送り方という運用上の判断を
// キットが黙って決めることになります。典型的な組み合わせは次の 2 つです。
//
//	// Vertex AI: gs:// はそのまま参照し、それ以外は取得してインライン
//	generator.New(client, generator.NewResolverChain(generator.NewGCSResolver(), fetchResolver))
//
//	// Gemini API: File API へ上げて使い回し、失敗したら取得してインライン
//	generator.New(client, generator.NewResolverChain(fileAPIResolver, fetchResolver))
//
// バックエンド判定は gemini.BackendInspector をオプショナルインターフェースとして
// 探ります。安全設定と人物生成の既定値がこの判定で切り替わり、Vertex AI 専用の
// resolver に Gemini API のクライアントを組み合わせた取り違えは ErrVertexAIRequired に
// なります。バックエンドを申告しないクライアントは弾きません。
func New(client gemini.Generator, resolver ports.ReferenceResolver, opts ...Option) (*Generator, error) {
	if client == nil {
		return nil, ErrAIClientRequired
	}
	if resolver == nil {
		return nil, ErrResolverRequired
	}

	inspector, declared := client.(gemini.BackendInspector)
	isVertexAI := declared && inspector.IsVertexAI()

	// Vertex 専用の resolver を弾くのは、バックエンドが Vertex でないと**申告された**
	// ときだけです。申告が無いクライアント（テスト用フェイクや、判定を転送しない
	// ラッパー）は素通しします。「Vertex でない証拠が無い」ことを「Vertex でない」と
	// 断定すると、実クライアント以外では GCSResolver が一切使えなくなるためです。
	if v, ok := resolver.(vertexOnlyResolver); ok && v.requiresVertexAI() && declared && !isVertexAI {
		return nil, ErrVertexAIRequired
	}

	g := &Generator{
		aiClient:       client,
		resolver:       resolver,
		isVertexAI:     isVertexAI,
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

// Generate は、参照画像（0〜複数）と構成パラメータに基づいて 1 枚の画像を生成します。
// WithRateLimit / WithRequestTimeout が設定されていれば、この呼び出しにも適用されます。
func (g *Generator) Generate(ctx context.Context, req ports.ImageRequest) (*ports.ImageResponse, error) {
	// 検証はレート制限の待機より前に行う。送信しないと決まったリクエストを発射枠で
	// 待たせると、発射間隔がそのまま失敗の検出遅れになる（60s 間隔の設定なら、
	// モデル名が空のリクエスト 1 件を弾くのに 60 秒掛かった上、発射枠も 1 つ消えていた）。
	prepared, err := g.prepare(req.GenerationOptions)
	if err != nil {
		return nil, err
	}

	// レート制限の待機はリクエストタイムアウトの外側で行う。待たされた時間を
	// 1 回あたりの上限に含めると、混雑がそのままタイムアウトに化けてしまう。
	if g.limiter != nil {
		if err := g.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	ctx, cancel := g.requestContext(ctx)
	defer cancel()

	// 参照の解決は resolver 次第で I/O を伴うため、検証と違って前倒しできない。
	// リクエストタイムアウトの内側で行う。
	attachments, err := g.collectImageAttachments(ctx, req.Images)
	if err != nil {
		return nil, err
	}
	return g.executeRequest(ctx, prepared.model, prepared.prompt, attachments, prepared.opts)
}

// GenerateBatch は複数のリクエストを設定された並列度・レート制限の下で実行します。
//
// 結果は入力と同じ順序で返します。一部の失敗で残りを打ち切らず、成功した結果も
// 破棄しません — 画像 1 枚ごとに生成コストが掛かっており、支払い済みの成果物を
// 1 件の失敗で捨てないためです。失敗した位置の要素は nil になり、エラーは
// requests[i] の添字を付けて集約されます（呼び出し側は結果とエラーの両方を見てください）。
func (g *Generator) GenerateBatch(ctx context.Context, reqs []ports.ImageRequest) ([]*ports.ImageResponse, error) {
	results := make([]*ports.ImageResponse, len(reqs))
	errs := make([]error, len(reqs))

	sem := make(chan struct{}, max(g.maxConcurrency, 1))

	// notStarted は、呼び出し側の context 終了により開始しなかった分の理由です。
	var notStarted error
	skipped := 0

	var wg sync.WaitGroup
	for i := range reqs {
		// 呼び出し側の context が終了していたら新しい生成は始めない
		// （進行中のものはそれぞれの ctx で打ち切られる）。
		if err := ctx.Err(); err != nil {
			notStarted = err
			skipped++
			continue
		}
		// セマフォは goroutine の内側ではなく、起動の手前で取る。内側で取ると
		// 並列度が 1 でも件数ぶんの goroutine が起きて待機するだけになる。
		sem <- struct{}{}

		wg.Go(func() {
			defer func() { <-sem }()
			results[i], errs[i] = g.Generate(ctx, reqs[i])
		})
	}
	wg.Wait()

	return results, joinBatchErrors(errs, notStarted, skipped)
}

// joinBatchErrors は各リクエストのエラーを添字付きで集約します。
//
// 添字を添えるのは、失敗したのが何番目のリクエストかを示す情報が他に無いためです
// （参照画像の添字 images[i] では、どのリクエストの話か分かりません）。
//
// 開始前に打ち切られた分は 1 件に畳みます。100 件のバッチをキャンセルすると、
// 同じ context.Canceled が 100 行並ぶだけになるためです。
func joinBatchErrors(errs []error, notStarted error, skipped int) error {
	joined := make([]error, 0, len(errs)+1)
	for i, err := range errs {
		if err == nil {
			continue
		}
		joined = append(joined, fmt.Errorf("requests[%d]: %w", i, err))
	}
	if notStarted != nil {
		joined = append(joined, fmt.Errorf("imagekit: %d request(s) not started: %w", skipped, notStarted))
	}
	return errors.Join(joined...)
}

// requestContext は WithRequestTimeout を適用した context を返します。
func (g *Generator) requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if g.requestTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, g.requestTimeout)
}

// preparedRequest は、検証と組み立てを終えて送信できる状態になったリクエストです。
// 参照画像だけは I/O を伴うため含まず、Generate が解決して足します。
type preparedRequest struct {
	model  string
	prompt string
	opts   gemini.GenerateOptions
}

// prepare はリクエストを検証し、送信用の形へ組み立てます。I/O は行いません。
func (g *Generator) prepare(req ports.GenerationOptions) (preparedRequest, error) {
	if req.Model == "" {
		return preparedRequest{}, ErrModelRequired
	}
	finalPrompt := buildFinalPrompt(req.Prompt, req.NegativePrompt)
	if finalPrompt == "" {
		return preparedRequest{}, ErrEmptyPrompt
	}
	// シードを生成側で決めるのは送信前の1回だけ。executeRequest は opts.Seed を
	// そのまま UsedSeed として返すため、ここで埋めた値が呼び出し側に届く。
	if g.autoSeed && req.Seed == nil {
		req.Seed = newSeed()
	}

	return preparedRequest{
		model:  req.Model,
		prompt: finalPrompt,
		opts:   g.toOptions(req),
	}, nil
}
