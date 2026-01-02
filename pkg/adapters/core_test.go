package adapters

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
	"google.golang.org/genai"
)

// --- Mocks ---

// mockHTTPClient は httpkit.ClientInterface を実装します。
type mockHTTPClient struct {
	fetchFunc func(ctx context.Context, url string) ([]byte, error)
}

func (m *mockHTTPClient) FetchBytes(ctx context.Context, url string) ([]byte, error) {
	return m.fetchFunc(ctx, url)
}

// インターフェースを満たすための空実装群なのだ
func (m *mockHTTPClient) DoRequest(req *http.Request) ([]byte, error) {
	return nil, nil
}

func (m *mockHTTPClient) FetchAndDecodeJSON(ctx context.Context, url string, v any) error {
	return nil
}

func (m *mockHTTPClient) PostJSONAndFetchBytes(ctx context.Context, url string, data any) ([]byte, error) {
	return nil, nil
}

func (m *mockHTTPClient) PostRawBodyAndFetchBytes(ctx context.Context, url string, body []byte, contentType string) ([]byte, error) {
	return nil, nil
}

// mockCache は ImageCacher インターフェースを実装するのだ。
type mockCache struct {
	data map[string]interface{}
}

func (m *mockCache) Get(key string) (interface{}, bool) {
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
	// PNGの最小構成バイナリ（シグネチャ含む）
	validPng := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90w\x53\xde")

	t.Run("キャッシュにある場合はキャッシュから取得して返すのだ", func(t *testing.T) {
		cache := &mockCache{data: map[string]interface{}{"http://test.com/img.png": validPng}}
		core := NewGeminiImageCore(nil, cache, time.Hour)

		part := core.PrepareImagePart(ctx, "http://test.com/img.png")

		if part == nil || part.InlineData == nil {
			t.Fatal("キャッシュから画像が取得できなかったのだ")
		}
		if !reflect.DeepEqual(part.InlineData.Data, validPng) {
			t.Errorf("取得したデータがキャッシュのものと一致しないのだ")
		}
	})

	t.Run("キャッシュにない場合はDLして保存するのだ", func(t *testing.T) {
		cache := &mockCache{data: make(map[string]interface{})}
		httpClient := &mockHTTPClient{
			fetchFunc: func(ctx context.Context, url string) ([]byte, error) {
				return validPng, nil
			},
		}
		core := NewGeminiImageCore(httpClient, cache, time.Hour)

		part := core.PrepareImagePart(ctx, "http://test.com/new.png")

		if part == nil {
			t.Fatal("画像の生成に失敗したのだ")
		}
		if _, found := cache.Get("http://test.com/new.png"); !found {
			t.Error("ダウンロードした画像がキャッシュに保存されていないのだ")
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
					},
				},
			},
		}

		out, err := core.ParseToResponse(resp, seed)
		if err != nil {
			t.Fatalf("パース中にエラーが発生したのだ: %v", err)
		}
		if string(out.Data) != "dummy-data" || out.UsedSeed != seed {
			t.Error("抽出データまたはシード値が想定と異なるのだ")
		}
	})

	t.Run("異常系: FinishReason が異常（SAFETY等）な場合", func(t *testing.T) {
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
			t.Error("異常な FinishReason のときはエラーを返すべきなのだ")
		}
	})
}
