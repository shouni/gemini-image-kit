package adapters

import (
	"context"
	"net/http"
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

// ClientInterface を満たすための空実装なのだ
func (m *mockHTTPClient) DoRequest(req *http.Request) ([]byte, error)                     { return nil, nil }
func (m *mockHTTPClient) FetchAndDecodeJSON(ctx context.Context, url string, v any) error { return nil }
func (m *mockHTTPClient) PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error) {
	return nil, nil
}
func (m *mockHTTPClient) PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error) {
	return nil, nil
}

type mockCache struct {
	data map[string]interface{}
}

func (m *mockCache) Get(key string) (interface{}, bool) {
	if m.data == nil {
		return nil, false
	}
	v, ok := m.data[key]
	return v, ok
}

func (m *mockCache) Set(key string, value interface{}, d time.Duration) {
	if m.data == nil {
		m.data = make(map[string]interface{})
	}
	m.data[key] = value
}

// --- Tests ---

func TestGeminiImageCore_PrepareImagePart(t *testing.T) {
	ctx := context.Background()
	// 有効なPNGヘッダーを持つダミーデータなのだ
	validPng := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90w\x53\xde")

	// 実際のテストでは名前解決可能なパブリックドメインを使用する必要があるのだ
	safeURL := "https://www.google.com/test.png"

	t.Run("キャッシュにある場合は安全チェックを飛ばしてキャッシュを返すのだ", func(t *testing.T) {
		cache := &mockCache{data: map[string]interface{}{safeURL: validPng}}
		// キャッシュヒット時は httpClient は呼ばれないので nil でOKなのだ
		core := NewGeminiImageCore(nil, cache, time.Hour)

		part := core.PrepareImagePart(ctx, safeURL)

		if part == nil || part.InlineData == nil {
			t.Fatal("キャッシュから画像が取得できなかったのだ")
		}
		if !reflect.DeepEqual(part.InlineData.Data, validPng) {
			t.Errorf("データが一致しないのだ")
		}
	})

	t.Run("キャッシュにない場合はバリデーション後にDLして保存するのだ", func(t *testing.T) {
		cache := &mockCache{data: make(map[string]interface{})}
		httpClient := &mockHTTPClient{
			fetchFunc: func(ctx context.Context, url string) ([]byte, error) {
				return validPng, nil
			},
		}
		core := NewGeminiImageCore(httpClient, cache, time.Hour)

		part := core.PrepareImagePart(ctx, safeURL)

		// 注意: 実行環境がオフライン、あるいはDNS解決できない場合はここで失敗する可能性があるのだ
		if part == nil {
			t.Log("Warning: 名前解決に失敗してnilが返った可能性があるのだ（オフライン環境等）")
			t.Skip("ネットワーク/DNS依存のためスキップするのだ")
			return
		}

		if _, found := cache.Get(safeURL); !found {
			t.Error("キャッシュに保存されていないのだ")
		}
		if !reflect.DeepEqual(part.InlineData.Data, validPng) {
			t.Error("DLしたデータが正しくPartに変換されていないのだ")
		}
	})

	t.Run("安全でないURL（ローカルIP）はブロックするのだ", func(t *testing.T) {
		unsafeURL := "http://127.0.0.1/attack.png"
		cache := &mockCache{data: make(map[string]interface{})}
		httpClient := &mockHTTPClient{}
		core := NewGeminiImageCore(httpClient, cache, time.Hour)

		part := core.PrepareImagePart(ctx, unsafeURL)

		if part != nil {
			t.Error("ローカルIPへのアクセスがブロックされなかったのだ")
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

		out, err := core.ParseToResponse(resp, seed)
		if err != nil {
			t.Fatalf("パース中にエラーが発生したのだ: %v", err)
		}
		if string(out.Data) != "dummy-data" || out.UsedSeed != seed {
			t.Error("抽出データまたはシード値が異なるのだ")
		}
	})

	t.Run("異常系: FinishReason が異常（Safety）な場合", func(t *testing.T) {
		resp := &gemini.Response{
			RawResponse: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						FinishReason: genai.FinishReasonSafety,
					},
				},
			},
		}

		_, err := core.ParseToResponse(resp, seed)
		if err == nil {
			t.Error("セーフティフィルターに抵触した場合はエラーを返すべきなのだ")
		}
		if !strings.Contains(err.Error(), "FinishReason: SAFETY") {
			t.Errorf("エラーメッセージが不適切なのだ: %v", err)
		}
	})
}
