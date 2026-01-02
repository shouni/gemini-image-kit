package adapters

import (
	"context"
	"strings"
	"testing"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// --- Mocks ---

// mockImageCore は adapters.ImageGeneratorCore を実装するのだ
type mockImageCore struct {
	prepareFunc func(url string) *genai.Part
	parseFunc   func(resp *gemini.Response, seed int64) (*ImageOutput, error)
}

func (m *mockImageCore) PrepareImagePart(ctx context.Context, url string) *genai.Part {
	return m.prepareFunc(url)
}
func (m *mockImageCore) ToPart(data []byte) *genai.Part { return nil }
func (m *mockImageCore) ParseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error) {
	return m.parseFunc(resp, seed)
}

// mockAIClient は gemini.GenerativeModel インターフェースを完全に実装するのだ
type mockAIClient struct {
	generateFunc func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error)
}

// 今回のメイン検証対象メソッドなのだ
func (m *mockAIClient) GenerateWithParts(ctx context.Context, model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
	return m.generateFunc(model, parts, opts)
}

// インターフェースを満たすために必要な追加メソッドなのだ（空実装でOK）
func (m *mockAIClient) GenerateContent(ctx context.Context, model string, prompt string) (*gemini.Response, error) {
	return nil, nil
}

// --- Tests ---

func TestGeminiImageAdapter_GenerateMangaPanel(t *testing.T) {
	ctx := context.Background()
	modelName := "imagen-3.0"
	style := "anime style, high quality"

	t.Run("正常系: プロンプトとオプションが正しくAIクライアントに渡されるのだ", func(t *testing.T) {
		seedValue := int32(1234)
		req := domain.ImageGenerationRequest{
			Prompt:      "ずんだもんが走る",
			AspectRatio: "16:9",
			Seed:        &seedValue,
		}

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				// プロンプト結合の検証
				if !strings.Contains(parts[0].Text, req.Prompt) || !strings.Contains(parts[0].Text, style) {
					t.Errorf("プロンプトが正しく結合されていないのだ: %s", parts[0].Text)
				}
				// オプション伝搬の検証
				if opts.AspectRatio != req.AspectRatio || opts.Seed == nil || *opts.Seed != seedValue {
					t.Error("オプションが正しく渡されていないのだ")
				}
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		core := &mockImageCore{
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return &ImageOutput{Data: []byte("fake-image"), MimeType: "image/png", UsedSeed: seed}, nil
			},
		}

		adapter, err := NewGeminiImageAdapter(core, ai, modelName, style)
		if err != nil {
			t.Fatalf("アダプターの生成に失敗したのだ: %v", err)
		}

		resp, err := adapter.GenerateMangaPanel(ctx, req)
		if err != nil {
			t.Fatalf("エラーが発生したのだ: %v", err)
		}
		if string(resp.Data) != "fake-image" || resp.UsedSeed != int64(seedValue) {
			t.Error("レスポンスデータまたはシードが不正なのだ")
		}
	})

	t.Run("参照画像がある場合にパーツに追加されるのだ", func(t *testing.T) {
		req := domain.ImageGenerationRequest{
			Prompt:       "ポーズをとる",
			ReferenceURL: "http://example.com/ref.png",
		}

		coreCalled := false
		core := &mockImageCore{
			prepareFunc: func(url string) *genai.Part {
				coreCalled = true
				return &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("ref")}}
			},
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return &ImageOutput{}, nil
			},
		}

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				// テキスト + 画像 の2パーツあることを検証
				if len(parts) != 2 {
					t.Errorf("パーツ数が不正なのだ: %d", len(parts))
				}
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		adapter, _ := NewGeminiImageAdapter(core, ai, modelName, style)
		_, _ = adapter.GenerateMangaPanel(ctx, req)

		if !coreCalled {
			t.Error("参照画像の処理が呼ばれなかったのだ")
		}
	})
}
