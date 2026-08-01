package generator

import (
	"math"
	"math/rand/v2"
	"strings"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

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

// newSeed は WithAutoSeed 用の乱数シードを返します。
//
// go-gemini-client が int32 の範囲外を弾く（ErrInvalidSeed）ため、範囲内に収めます。
func newSeed() *int64 {
	seed := rand.Int64N(math.MaxInt32)
	return &seed
}
