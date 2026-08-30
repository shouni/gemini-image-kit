// Package generator は、Gemini API / Vertex AI 上の画像モデルによる画像生成を実行します。
//
// 参照画像の送り方は ports.ReferenceResolver として外から注入します。gs:// をそのまま
// 参照するのか、File API へ上げて使い回すのか、取得してインラインで送るのかは
// バックエンドと運用で変わるためです。依存（取得・キャッシュ）は選んだ resolver だけが
// 要求するので、gs:// しか使わない構成では何も渡さずに済みます。
package generator

import (
	"context"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

// Generator がポートを満たすことをコンパイル時に保証します。
var _ ports.ImageGenerator = (*Generator)(nil)

// Generator は画像生成の実装です。
//
// 発射間隔・上限時間・重複排除といった呼び出しガードは持ちません。クォータは
// プロジェクト単位で操作の種類ごとではないため、画像生成だけを絞ってもテキスト生成が
// 同じクォータを食い尽くせてしまうからです。ガードは ports.ImageGenerator を
// go-gemini-client/callguard でデコレートし、テキスト生成と 1 つの Guard を
// 共有する形でワークフロー層に置いてください。
type Generator struct {
	aiClient   gemini.Generator
	resolver   ports.ReferenceResolver
	isVertexAI bool
	autoSeed   bool
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
// 探ります。安全設定と人物生成の既定値がこの判定で切り替わり、バックエンド制約を
// 持つ resolver との取り違えは ErrVertexAIRequired / ErrGeminiAPIRequired になります。
// バックエンドを申告しないクライアントは弾きません。
func New(client gemini.Generator, resolver ports.ReferenceResolver, opts ...Option) (*Generator, error) {
	if client == nil {
		return nil, ErrAIClientRequired
	}
	if resolver == nil {
		return nil, ErrResolverRequired
	}

	inspector, declared := client.(gemini.BackendInspector)
	isVertexAI := declared && inspector.IsVertexAI()

	// 素通しするのは、「証拠が無い」ことを「食い違っている」と断定すると、実クライアント
	// 以外（テスト用フェイクや、判定を転送しないラッパー）では制約付き resolver が
	// 一切使えなくなるためです。
	switch requiredBackend(resolver) {
	case backendVertexAI:
		if declared && !isVertexAI {
			return nil, ErrVertexAIRequired
		}
	case backendGeminiAPI:
		if isVertexAI {
			return nil, ErrGeminiAPIRequired
		}
	}

	g := &Generator{
		aiClient:   client,
		resolver:   resolver,
		isVertexAI: isVertexAI,
		autoSeed:   true,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(g)
		}
	}
	return g, nil
}

// Generate は、参照画像（0〜複数）と構成パラメータに基づいて 1 枚の画像を生成します。
// 打ち切りは呼び出し側の context にのみ従います。
func (g *Generator) Generate(ctx context.Context, req ports.ImageRequest) (*ports.ImageResponse, error) {
	// 検証（prepare）は参照の解決より前に行う。送信しないと決まったリクエストの
	// ために参照画像を取得・アップロードすると、捨てる結果に I/O を払うことになる。
	prepared, err := g.prepare(req.GenerationOptions)
	if err != nil {
		return nil, err
	}

	attachments, err := g.collectImageAttachments(ctx, req.Images)
	if err != nil {
		return nil, err
	}
	return g.executeRequest(ctx, prepared.model, prepared.prompt, attachments, prepared.opts)
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
