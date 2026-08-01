package generator

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"sync"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/imgutil"
	"github.com/shouni/gemini-image-kit/ports"
)

// generate は画像生成のコアロジックです。
func (g *GeminiGenerator) generate(ctx context.Context, req ports.GenerationOptions, uris []ports.ImageURI) (*ports.ImageResponse, error) {
	if req.Model == "" {
		return nil, ErrModelRequired
	}
	finalPrompt := buildFinalPrompt(req.Prompt, req.NegativePrompt)
	if finalPrompt == "" {
		return nil, ErrEmptyPrompt
	}
	// シードを生成側で決めるのは送信前の1回だけ。ExecuteRequest は opts.Seed を
	// そのまま UsedSeed として返すため、ここで埋めた値が呼び出し側に届く。
	if g.autoSeed && req.Seed == nil {
		req.Seed = newSeed()
	}

	// 1. 画像アセット（素材）を収集
	attachments, err := g.collectImageAttachments(ctx, uris)
	if err != nil {
		return nil, err
	}

	// 2. ImageSize を含めたオプション構築
	opts := g.toOptions(req)
	return g.core.ExecuteRequest(ctx, req.Model, finalPrompt, attachments, opts)
}

// collectImageAttachments は ImageURI 構造体から添付を生成します。
//
// 参照画像の解決は GCS / HTTP の往復を伴うため並行に実行します。融合生成
// (GenerateFusedImage) では参照が増えるほど直列の待ち時間がそのまま積み上がるためです。
// 結果は入力順のまま返します（参照画像の並び順はモデルの解釈に影響します）。
//
// 並行実行するため、注入する ports.ImageCacher は同時アクセス安全である必要があります。
func (g *GeminiGenerator) collectImageAttachments(ctx context.Context, uris []ports.ImageURI) ([]gemini.Attachment, error) {
	if len(uris) <= 1 {
		return g.collectSequentially(ctx, uris)
	}

	resolved := make([]gemini.Attachment, len(uris))
	errs := make([]error, len(uris))

	// 1つでも失敗したら残りの取得は無駄になるので打ち切る。
	fetchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i, uri := range uris {
		wg.Add(1)
		go func() {
			defer wg.Done()
			attachment, err := g.resolveImageAttachment(fetchCtx, uri)
			if err != nil {
				errs[i] = err
				cancel()
				return
			}
			resolved[i] = attachment
		}()
	}
	wg.Wait()

	if err := firstMeaningfulError(errs); err != nil {
		return nil, err
	}

	attachments := make([]gemini.Attachment, 0, len(resolved))
	for _, attachment := range resolved {
		// 参照先を持たない要素は送るものが無いので落とす。
		if !attachment.IsEmpty() {
			attachments = append(attachments, attachment)
		}
	}
	return attachments, nil
}

// collectSequentially は参照が 1 枚以下の場合の経路です。goroutine を起こす意味が
// ないだけでなく、単一参照の失敗がそのまま素のエラーとして返るようにもなります。
func (g *GeminiGenerator) collectSequentially(ctx context.Context, uris []ports.ImageURI) ([]gemini.Attachment, error) {
	attachments := make([]gemini.Attachment, 0, len(uris))
	for _, uri := range uris {
		attachment, err := g.resolveImageAttachment(ctx, uri)
		if err != nil {
			return nil, err
		}
		if !attachment.IsEmpty() {
			attachments = append(attachments, attachment)
		}
	}
	return attachments, nil
}

// firstMeaningfulError は、並行実行の結果から報告すべきエラーを 1 つ選びます。
//
// 入力順で最初のものを選ぶのは、どの参照で失敗したかが実行ごとに変わらないように
// するためです。打ち切り (context.Canceled) は最初の失敗の二次的な結果なので、
// 本来の失敗が他にあるならそちらを優先します。
func firstMeaningfulError(errs []error) error {
	for _, err := range errs {
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	// 呼び出し側の context が終了した場合など、すべてが打ち切り由来だったケース。
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// newSeed は WithAutoSeed 用の乱数シードを返します。
//
// go-gemini-client が int32 の範囲外を弾く（ErrInvalidSeed）ため、範囲内に収めます。
func newSeed() *int64 {
	seed := rand.Int64N(math.MaxInt32)
	return &seed
}

// resolveImageAttachment は ImageURI から添付を生成します。
func (g *GeminiGenerator) resolveImageAttachment(ctx context.Context, uri ports.ImageURI) (gemini.Attachment, error) {
	if g.core.IsVertexAI() && IsGCSURI(uri.ReferenceURL) {
		return fileAttachment(uri.ReferenceURL, uri.ReferenceURL), nil
	}
	if uri.FileAPIURI != "" {
		return fileAttachment(uri.FileAPIURI, uri.ReferenceURL), nil
	}
	// 参照先が一切設定されていない要素は読み飛ばす（テキストのみの生成で使われる）。
	if uri.IsEmpty() {
		return gemini.Attachment{}, nil
	}
	attachment, err := g.core.PrepareImageAttachment(ctx, uri.ReferenceURL)
	if err != nil {
		return gemini.Attachment{}, fmt.Errorf("failed to prepare image attachment for %q: %w", uri.ReferenceURL, err)
	}
	return attachment, nil
}

// fileAttachment は URI 参照の添付を生成します。
//
// 拡張子から MIME type を判別できない場合は MIMEType を設定しません。
// 誤った型を申告するとサーバー側のデコードが失敗しうるため、
// 推測できないときはサーバーのコンテンツ判定に委ねます。
func fileAttachment(fileURI, mimeHintURI string) gemini.Attachment {
	return gemini.Attachment{URI: fileURI, MIMEType: imgutil.GuessMIMEType(mimeHintURI)}
}

// toOptions は Gemini へのリクエストオプションを構築します。
func (g *GeminiGenerator) toOptions(req ports.GenerationOptions) gemini.GenerateOptions {
	isVertex := g.core.IsVertexAI()
	opts := gemini.GenerateOptions{
		AspectRatio:     req.AspectRatio,
		ImageSize:       req.ImageSize,
		SystemPrompt:    req.SystemPrompt,
		Seed:            req.Seed,
		SafetySettings:  gemini.NewSafetySettings(safetyThreshold(isVertex)),
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		MaxOutputTokens: req.MaxOutputTokens,
		ThinkingBudget:  req.ThinkingBudget,
		ThinkingLevel:   req.ThinkingLevel,
	}

	// Vertex AI の場合のみ PersonGeneration を設定する
	// Gemini API (Google AI) ではこのフィールドが含まれると致命的エラーになるため
	if isVertex {
		opts.PersonGeneration = gemini.PersonGenerationAllowAll
		if req.PersonGeneration != gemini.PersonGenerationUnspecified {
			opts.PersonGeneration = req.PersonGeneration
		}
	}

	return opts
}

// safetyThreshold は、バックエンドごとの安全フィルタ閾値を返します。
// Vertex AI は OFF を受け付けないため、そちらでは BLOCK_NONE を使います。
func safetyThreshold(isVertex bool) gemini.SafetyThreshold {
	if isVertex {
		return gemini.SafetyBlockNone
	}
	return gemini.SafetyOff
}

// buildFinalPrompt はプロンプトと否定プロンプトを結合します。
func buildFinalPrompt(prompt, negative string) string {
	p := strings.TrimSpace(prompt)
	n := strings.TrimSpace(negative)

	if p == "" && n == "" {
		return ""
	}
	if n == "" {
		return p
	}

	var sb strings.Builder
	sb.WriteString(p)
	sb.WriteString(negativePromptSeparator)
	sb.WriteString(n)
	return sb.String()
}
