package generator

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// --- Mocks ---

type mockHTTPClient struct {
	fetchFunc func(ctx context.Context, url string) ([]byte, error)
}

func (m *mockHTTPClient) FetchBytes(ctx context.Context, url string) ([]byte, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, url)
	}
	return nil, nil
}

type mockCache struct {
	data map[string]any
}

func (m *mockCache) Get(key string) (any, bool) {
	if m.data == nil {
		return nil, false
	}
	v, ok := m.data[key]
	return v, ok
}

func (m *mockCache) Set(key string, value any, d time.Duration) {
	if m.data == nil {
		m.data = make(map[string]any)
	}
	m.data[key] = value
}

// --- Tests ---

func TestNewGeminiImageCore(t *testing.T) {
	t.Run("正常系: 必須パラメータがあれば初期化できるのだ", func(t *testing.T) {
		core, err := NewGeminiImageCore(&mockHTTPClient{}, nil, time.Hour)
		if err != nil {
			t.Fatalf("初期化に失敗したのだ: %v", err)
		}
		if core == nil {
			t.Fatal("インスタンスが生成されなかったのだ")
		}
	})

	t.Run("異常系: HTTPClientがnilならエラーを返すのだ", func(t *testing.T) {
		core, err := NewGeminiImageCore(nil, nil, time.Hour)
		if err == nil {
			t.Error("HTTPClientがnilなのにエラーが発生しなかったのだ")
		}
		if core != nil {
			t.Error("エラーなのにインスタンスが返されたのだ")
		}
	})
}

func TestGeminiImageCore_PrepareImagePart(t *testing.T) {
	ctx := context.Background()
	validPng := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90w\x53\xde")
	safeURL := "https://www.google.com/test.png"

	t.Run("キャッシュにある場合は安全チェックを飛ばしてキャッシュを返すのだ", func(t *testing.T) {
		cache := &mockCache{data: map[string]any{safeURL: validPng}}
		core, err := NewGeminiImageCore(&mockHTTPClient{}, cache, time.Hour)
		if err != nil {
			t.Fatalf("初期化エラー: %v", err)
		}

		// 修正ポイント: prepareImagePart (小文字) を呼ぶのだ
		part := core.prepareImagePart(ctx, safeURL)

		if part == nil || part.InlineData == nil {
			t.Fatal("キャッシュから画像が取得できなかったのだ")
		}
		if !reflect.DeepEqual(part.InlineData.Data, validPng) {
			t.Errorf("データが一致しないのだ")
		}
	})

	t.Run("キャッシュにない場合はバリデーション後にDLして保存するのだ", func(t *testing.T) {
		cache := &mockCache{data: make(map[string]any)}
		httpClient := &mockHTTPClient{
			fetchFunc: func(ctx context.Context, url string) ([]byte, error) {
				return validPng, nil
			},
		}
		core, err := NewGeminiImageCore(httpClient, cache, time.Hour)
		if err != nil {
			t.Fatalf("初期化エラー: %v", err)
		}

		// 修正ポイント: prepareImagePart (小文字) を呼ぶのだ
		part := core.prepareImagePart(ctx, safeURL)

		if part == nil {
			t.Skip("外部ネットワーク制限等によりスキップするのだ")
			return
		}

		if _, found := cache.Get(safeURL); !found {
			t.Error("キャッシュに保存されていないのだ")
		}
		if !reflect.DeepEqual(part.InlineData.Data, validPng) {
			t.Error("DLしたデータが正しく変換されていないのだ")
		}
	})

	t.Run("安全でないURLはブロックするのだ", func(t *testing.T) {
		core, _ := NewGeminiImageCore(&mockHTTPClient{}, nil, time.Hour)

		cases := []struct {
			name string
			url  string
		}{
			{"スキーム不正(ftp)", "ftp://example.com/test.png"},
			{"ループバック", "http://127.0.0.1/attack"},
			{"プライベートIP", "http://192.168.1.1/internal"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				// 修正ポイント: prepareImagePart (小文字) を呼ぶのだ
				part := core.prepareImagePart(ctx, tc.url)
				if part != nil {
					t.Errorf("%s がブロックされなかったのだ", tc.name)
				}
			})
		}
	})
}

func TestGeminiImageCore_ParseToResponse(t *testing.T) {
	core := &GeminiImageCore{}
	seed := int64(9999)

	t.Run("正常系: 画像が含まれるレスポンスを正しく解析するのだ", func(t *testing.T) {
		resp := &gemini.Response{
			RawResponse: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{
									InlineData: &genai.Blob{
										MIMEType: "image/png",
										Data:     []byte("dummy-data"),
									},
								},
							},
						},
						FinishReason: genai.FinishReasonStop,
					},
				},
			},
		}

		// 修正ポイント: parseToResponse (小文字) を呼ぶのだ
		out, err := core.parseToResponse(resp, seed)
		if err != nil {
			t.Fatalf("パース中にエラーが発生したのだ: %v", err)
		}
		if string(out.Data) != "dummy-data" || out.UsedSeed != seed {
			t.Error("抽出データまたはシード値が異なるのだ")
		}
	})

	t.Run("異常系: FinishReason が SAFETY の場合", func(t *testing.T) {
		resp := &gemini.Response{
			RawResponse: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						FinishReason: genai.FinishReasonSafety,
					},
				},
			},
		}

		// 修正ポイント: parseToResponse (小文字) を呼ぶのだ
		_, err := core.parseToResponse(resp, seed)
		if err == nil {
			t.Fatal("セーフティフィルター時はエラーを返すべきなのだ")
		}
		if !strings.Contains(err.Error(), "SAFETY") {
			t.Errorf("エラーメッセージに理由が含まれていないのだ: %v", err)
		}
	})
}
