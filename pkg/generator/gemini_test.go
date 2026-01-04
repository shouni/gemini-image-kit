package generator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

func (m *mockImageCore) prepareImagePart(ctx context.Context, url string) *genai.Part {
	if m.prepareFunc != nil {
		return m.prepareFunc(ctx, url)
	}
	return nil
}

func (m *mockImageCore) toPart(data []byte) *genai.Part {
	if m.toPartFunc != nil {
		return m.toPartFunc(data)
	}
	return nil
}

func (m *mockImageCore) parseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error) {
	if m.parseFunc != nil {
		return m.parseFunc(resp, seed)
	}
	return nil, nil
}

// --- Tests ---

func TestGeminiGenerator_GenerateMangaPanel(t *testing.T) {
	ctx := context.Background()
	modelName := "imagen-3.0"

	t.Run("成功: 正しいプロンプトとシードがAIクライアントに渡されるのだ", func(t *testing.T) {
		var seedVal int64 = 777
		req := domain.ImageGenerationRequest{
			Prompt:      "ずんだもん、走る",
			AspectRatio: "1:1",
			Seed:        &seedVal,
		}

		ai := &mockAIClient{
			generateWithPartsFunc: func(ctx context.Context, model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				if parts[0].Text != req.Prompt {
					t.Errorf("prompt mismatch: got %s", parts[0].Text)
				}
				if opts.Seed == nil || *opts.Seed != int32(seedVal) {
					t.Errorf("seed conversion failed: got %v", opts.Seed)
				}
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		core := &mockImageCore{
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return &ImageOutput{Data: []byte("fake-png"), MimeType: "image/png", UsedSeed: seed}, nil
			},
		}

		gen, _ := NewGeminiGenerator(core, ai, modelName)
		resp, err := gen.GenerateMangaPanel(ctx, req)

		if err != nil {
			t.Fatalf("error should be nil: %v", err)
		}
		if resp.UsedSeed != seedVal {
			t.Errorf("expected seed %d, got %d", seedVal, resp.UsedSeed)
		}
	})
}

func TestGeminiGenerator_GenerateMangaPage(t *testing.T) {
	ctx := context.Background()
	modelName := "imagen-3.0"

	t.Run("成功: 複数の参照画像URLがすべてパーツに追加されるのだ", func(t *testing.T) {
		req := domain.ImagePageRequest{
			Prompt:        "対決シーン",
			ReferenceURLs: []string{"url1", "url2"},
		}

		ai := &mockAIClient{
			generateWithPartsFunc: func(ctx context.Context, model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				// テキスト(1) + 画像(2) = 3パーツあるはずなのだ
				if len(parts) != 3 {
					t.Errorf("expected 3 parts, got %d", len(parts))
				}
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		core := &mockImageCore{
			prepareFunc: func(ctx context.Context, url string) *genai.Part {
				// URLごとにダミーのPartを返すのだ
				return &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png"}}
			},
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return &ImageOutput{Data: []byte("page-png")}, nil
			},
		}

		gen, _ := NewGeminiGenerator(core, ai, modelName)
		_, err := gen.GenerateMangaPage(ctx, req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("失敗: AIクライアントのエラーが適切にラップされて返るのだ", func(t *testing.T) {
		expectedErr := errors.New("ai error")
		ai := &mockAIClient{
			generateWithPartsFunc: func(ctx context.Context, model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				return nil, expectedErr
			},
		}
		core := &mockImageCore{}

		gen, _ := NewGeminiGenerator(core, ai, modelName)
		_, err := gen.GenerateMangaPage(ctx, domain.ImagePageRequest{})

		if err == nil || !strings.Contains(err.Error(), "Gemini一括ページ生成エラー") {
			t.Errorf("error should contain context message: %v", err)
		}
	})
}

func TestNewGeminiGenerator(t *testing.T) {
	t.Run("nilチェック: 依存関係が足りない場合はエラーを返すのだ", func(t *testing.T) {
		_, err := NewGeminiGenerator(nil, nil, "model")
		if err == nil {
			t.Error("expected error for nil dependencies")
		}
	})
}
