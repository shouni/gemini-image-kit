package adapters

import (
	"context"
	"testing"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

func TestGeminiMangaPageAdapter_GenerateMangaPage(t *testing.T) {
	ctx := context.Background()
	modelName := "imagen-3.0"

	t.Run("正常系: 複数の画像URLが正しくパーツに追加されるのだ", func(t *testing.T) {
		req := domain.ImagePageRequest{
			Prompt: "豪華なマンガの1ページ",
			ReferenceURLs: []string{
				"http://example.com/chara1.png",
				"http://example.com/chara2.png",
			},
			AspectRatio: "3:4",
		}

		// 画像が2回呼ばれることを期待する mockImageCore
		prepareCallCount := 0
		core := &mockImageCore{
			prepareFunc: func(url string) *genai.Part {
				prepareCallCount++
				return &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("img")}}
			},
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return &ImageOutput{Data: []byte("final-page"), MimeType: "image/png", UsedSeed: seed}, nil
			},
		}

		// パーツ構成を検証する mockAIClient
		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				// テキスト1つ + 画像2つ = 合計3パーツなのだ
				if len(parts) != 3 {
					t.Errorf("パーツ数が正しくないのだ。期待: 3, 実際: %d", len(parts))
				}
				if parts[0].Text != req.Prompt {
					t.Error("最初のパーツにプロンプトがセットされていないのだ")
				}
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		adapter := NewGeminiMangaPageAdapter(core, ai, modelName)
		resp, err := adapter.GenerateMangaPage(ctx, req)

		if err != nil {
			t.Fatalf("生成に失敗したのだ: %v", err)
		}
		if prepareCallCount != 2 {
			t.Errorf("画像準備の呼び出し回数が不正なのだ。期待: 2, 実際: %d", prepareCallCount)
		}
		if string(resp.Data) != "final-page" {
			t.Error("レスポンスデータが不正なのだ")
		}
	})

	t.Run("一部の画像DLに失敗しても残りで続行するのだ", func(t *testing.T) {
		req := domain.ImagePageRequest{
			Prompt: "一部失敗のテスト",
			ReferenceURLs: []string{
				"http://ok.com/image.png",
				"http://fail.com/bad.png", // これは失敗させるのだ
			},
		}

		core := &mockImageCore{
			prepareFunc: func(url string) *genai.Part {
				if url == "http://fail.com/bad.png" {
					return nil // 失敗をシミュレート
				}
				return &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("ok")}}
			},
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return &ImageOutput{Data: []byte("success-anyway")}, nil
			},
		}

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				// テキスト1つ + 成功した画像1つ = 合計2パーツなのだ
				if len(parts) != 2 {
					t.Errorf("失敗した画像がスキップされず、パーツ数が不正なのだ: %d", len(parts))
				}
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		adapter := NewGeminiMangaPageAdapter(core, ai, modelName)
		resp, err := adapter.GenerateMangaPage(ctx, req)

		if err != nil {
			t.Errorf("画像の一部失敗で生成が止まってしまったのだ: %v", err)
		}
		if string(resp.Data) != "success-anyway" {
			t.Error("データが不正なのだ")
		}
	})

	t.Run("空文字列のURLは無視されるのだ", func(t *testing.T) {
		req := domain.ImagePageRequest{
			Prompt:        "空URLチェック",
			ReferenceURLs: []string{"", "http://valid.com/img.png", ""},
		}

		prepareCallCount := 0
		core := &mockImageCore{
			prepareFunc: func(url string) *genai.Part {
				prepareCallCount++
				return &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("ok")}}
			},
			parseFunc: func(resp *gemini.Response, seed int64) (*ImageOutput, error) {
				return &ImageOutput{}, nil
			},
		}

		ai := &mockAIClient{
			generateFunc: func(model string, parts []*genai.Part, opts gemini.ImageOptions) (*gemini.Response, error) {
				return &gemini.Response{RawResponse: &genai.GenerateContentResponse{}}, nil
			},
		}

		adapter := NewGeminiMangaPageAdapter(core, ai, modelName)
		_, _ = adapter.GenerateMangaPage(ctx, req)

		if prepareCallCount != 1 {
			t.Errorf("空URLが正しくスキップされていないのだ。呼び出し回数: %d", prepareCallCount)
		}
	})
}
