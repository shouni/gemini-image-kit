package generator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-gemini-client/pkg/gemini"
	"github.com/shouni/go-remote-io/pkg/remoteio"
	"google.golang.org/genai"
)

// generate は画像生成のコアロジックです。
func (g *GeminiGenerator) generate(ctx context.Context, model, prompt, negative string, uris []domain.ImageURI, ar, size, sp string, seed *int64) (*domain.ImageResponse, error) {
	finalPrompt := buildFinalPrompt(prompt, negative)
	if finalPrompt == "" {
		return nil, fmt.Errorf("prompt cannot be empty")
	}

	// 1. 画像アセット（素材）を収集
	parts := g.collectImageParts(ctx, uris)

	// 2. 最後にテキストプロンプトを追加
	parts = append(parts, &genai.Part{Text: finalPrompt})

	// 3. ImageSize を含めたオプション構築
	opts := g.toOptions(ar, size, sp, seed)
	return g.core.ExecuteRequest(ctx, model, parts, opts)
}

// collectImageParts は ImageURI 構造体からパーツを生成します。
func (g *GeminiGenerator) collectImageParts(ctx context.Context, uris []domain.ImageURI) []*genai.Part {
	parts := make([]*genai.Part, 0, len(uris))

	for _, uri := range uris {
		// 1. Vertex AI モードで、GCS URI (gs://) の場合
		if g.core.IsVertexAI() && remoteio.IsGCSURI(uri.ReferenceURL) {
			parts = append(parts, &genai.Part{
				FileData: &genai.FileData{
					FileURI:  uri.ReferenceURL, // SDKの定義通り FileURI に gs:// パスを入れる
					MIMEType: g.guessMIMEType(uri.ReferenceURL),
				},
			})
			continue
		}

		// 2. Gemini File API URI がある場合 (Gemini API モード)
		if uri.FileAPIURI != "" {
			parts = append(parts, &genai.Part{
				FileData: &genai.FileData{
					FileURI:  uri.FileAPIURI,
					MIMEType: g.guessMIMEType(uri.ReferenceURL),
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

// guessMIMEType は拡張子から MIMEType を推測するヘルパー（実装例）
func (g *GeminiGenerator) guessMIMEType(uri string) string {
	ext := filepath.Ext(uri)
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg" // フォールバック
	}
}

// toOptions は Gemini へのリクエストオプションを構築します。
func (g *GeminiGenerator) toOptions(ar, size, sp string, seed *int64) gemini.GenerateOptions {
	return gemini.GenerateOptions{
		AspectRatio:  ar,
		ImageSize:    size,
		SystemPrompt: sp,
		Seed:         seed,
	}
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
