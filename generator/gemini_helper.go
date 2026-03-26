package generator

import (
	"context"
	"fmt"
	"strings"

	"github.com/shouni/go-gemini-client/gemini"
	"github.com/shouni/go-remote-io/remoteio"
	"google.golang.org/genai"

	"github.com/shouni/gemini-image-kit/imgutil"
	"github.com/shouni/gemini-image-kit/ports"
)

// generate は画像生成のコアロジックです。
func (g *GeminiGenerator) generate(ctx context.Context, req ports.GenerationOptions, uris []ports.ImageURI) (*ports.ImageResponse, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	finalPrompt := buildFinalPrompt(req.Prompt, req.NegativePrompt)
	if finalPrompt == "" {
		return nil, fmt.Errorf("prompt cannot be empty")
	}

	// 1. 画像アセット（素材）を収集
	parts := g.collectImageParts(ctx, uris)

	// 2. 最後にテキストプロンプトを追加
	parts = append(parts, &genai.Part{Text: finalPrompt})

	// 3. ImageSize を含めたオプション構築
	opts := g.toOptions(req.AspectRatio, req.ImageSize, req.SystemPrompt, req.Seed)
	return g.core.ExecuteRequest(ctx, req.Model, parts, opts)
}

// collectImageParts は ImageURI 構造体からパーツを生成します。
func (g *GeminiGenerator) collectImageParts(ctx context.Context, uris []ports.ImageURI) []*genai.Part {
	parts := make([]*genai.Part, 0, len(uris))

	for _, uri := range uris {
		// 1. Vertex AI モードで、GCS URI (gs://) の場合
		if g.core.IsVertexAI() && remoteio.IsGCSURI(uri.ReferenceURL) {
			parts = append(parts, &genai.Part{
				FileData: &genai.FileData{
					FileURI:  uri.ReferenceURL, // SDKの定義通り FileURI に gs:// パスを入れる
					MIMEType: imgutil.GuessMIMEType(uri.ReferenceURL),
				},
			})
			continue
		}

		// 2. Gemini File API URI がある場合 (Gemini API モード)
		if uri.FileAPIURI != "" {
			parts = append(parts, &genai.Part{
				FileData: &genai.FileData{
					FileURI:  uri.FileAPIURI,
					MIMEType: imgutil.GuessMIMEType(uri.ReferenceURL),
				},
			})
			continue
		}

		// 3. ローカルパスまたは HTTP URL の場合 (バイナリとして読み込み)
		if uri.ReferenceURL != "" {
			if res := g.core.PrepareImagePart(ctx, uri.ReferenceURL); res != nil {
				parts = append(parts, res)
			}
		}
	}
	return parts
}

// toOptions は Gemini へのリクエストオプションを構築します。
func (g *GeminiGenerator) toOptions(ar, size, sp string, seed *int64) gemini.GenerateOptions {
	// Vertex AI かどうかで安全設定の定数を切り替え
	threshold := genai.HarmBlockThresholdOff
	isVertex := g.core.IsVertexAI()

	if isVertex {
		threshold = genai.HarmBlockThresholdBlockNone
	}

	defaultSafetySettings := []*genai.SafetySetting{
		{Category: genai.HarmCategoryHarassment, Threshold: threshold},
		{Category: genai.HarmCategoryHateSpeech, Threshold: threshold},
		{Category: genai.HarmCategorySexuallyExplicit, Threshold: threshold},
		{Category: genai.HarmCategoryDangerousContent, Threshold: threshold},
	}

	// 基本となるオプションを構築
	opts := gemini.GenerateOptions{
		AspectRatio:    ar,
		ImageSize:      size,
		SystemPrompt:   sp,
		Seed:           seed,
		SafetySettings: defaultSafetySettings,
	}

	// Vertex AI の場合のみ PersonGeneration を設定する
	// Gemini API (Google AI) ではこのフィールドが含まれると致命的エラーになるため
	if isVertex {
		opts.PersonGeneration = gemini.PersonGenerationAllowAll
	}

	return opts
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
