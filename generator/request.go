package generator

import (
	"math"
	"math/rand/v2"
	"strings"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

// negativePromptSeparator は、ネガティブプロンプトをプロンプト本文へ連結する際の区切りです。
//
// **この描画は互換性の契約です。** ネガティブプロンプトは API のフィールドではなく、
// 下流のプロンプト実装（ap-story のデザインプロンプト等）がこの区切りの見た目に
// 依存しているため、変更しないでください。
const negativePromptSeparator = "\n\n[Negative Prompt]\n"

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

// toOptions は Gemini へのリクエストオプションを構築します。
//
// GenerationOptions は gemini.GenerateOptions を埋め込んでいるため、ここでの仕事は
// 呼び出し側が未指定の項目にバックエンド既定を補うことだけです。呼び出し側が明示
// した SafetySettings / PersonGeneration は尊重します（以前は無条件に上書きしており、
// 利用側が安全フィルタを厳しくする手段がありませんでした）。
func (g *Generator) toOptions(req ports.GenerationOptions) gemini.GenerateOptions {
	opts := req.GenerateOptions
	isVertex := g.isVertexAI

	if opts.SafetySettings == nil {
		opts.SafetySettings = gemini.NewSafetySettings(safetyThreshold(isVertex))
	}

	if isVertex {
		// キャラクター生成が主用途のため、未指定時は人物生成を許可する。
		if opts.PersonGeneration == gemini.PersonGenerationUnspecified {
			opts.PersonGeneration = gemini.PersonGenerationAllowAll
		}
	} else {
		// Gemini API (Google AI) バックエンドは PersonGeneration 未対応のため送らない。
		opts.PersonGeneration = gemini.PersonGenerationUnspecified
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

// newSeed は自動シード採番用の乱数シードを返します。
//
// go-gemini-client が int32 の範囲外を弾く（ErrInvalidSeed）ため、範囲内に収めます。
func newSeed() *int64 {
	seed := rand.Int64N(math.MaxInt32)
	return &seed
}
