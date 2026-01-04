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

// mockCache は ImageCacher インターフェースを []byte 型で実装するのだ。
type mockCache struct {
	data map[string][]byte
}

func (m *mockCache) Get(key string) ([]byte, bool) {
	if m.data == nil {
		return nil, false
	}
	v, ok := m.data[key]
	return v, ok
}

func (m *mockCache) Set(key string, value []byte, d time.Duration) {
	if m.data == nil {
		m.data = make(map[string][]byte)
	}
	m.data[key] = value
}

// --- Tests ---

func TestGeminiImageCore_PrepareImagePart(t *testing.T) {
	ctx := context.Background()
	// 有効なPNGヘッダーを持つダミーデータ
	validPng := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90w\x53\xde")
	safeURL := "https://www.google.com/test.png"

	t.Run("キャッシュにある場合は安全チェックを飛ばしてキャッシュを返すのだ", func(t *testing.T) {
		cache := &mockCache{data: map[string][]byte{safeURL: validPng}}
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
		cache := &mockCache{data: make(map[string][]byte)}
		httpClient := &mockHTTPClient{
			fetchFunc: func(ctx context.Context, url string) ([]byte, error) {
				return validPng, nil
			},
		}
		core := NewGeminiImageCore(httpClient, cache, time.Hour)

		part := core.PrepareImagePart(ctx, safeURL)

		// ネットワーク環境等で名前解決に失敗する場合はSkipするのだ
		if part == nil {
			t.Skip("外部ネットワークへの名前解決制限によりテストをスキップするのだ")
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
		core := NewGeminiImageCore(&mockHTTPClient{}, nil, time.Hour)

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
				part := core.PrepareImagePart(ctx, tc.url)
				if part != nil {
					t.Errorf("%s がブロックされなかったのだ", tc.name)
				}
			})
		}
	})
}

func TestIsSafeURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"正常なパブリックURL", "https://www.google.com/favicon.ico", false},
		{"不正なスキーム", "gopher://example.com", true},
		{"ループバック", "http://localhost/admin", true},
		{"プライベートIP (クラスA)", "http://10.255.255.254/metadata", true},
		{"名前解決できないドメイン", "http://this.should.not.exist.invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			safe, err := isSafeURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("isSafeURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !safe {
				t.Error("safe URL was flagged as unsafe")
			}
			if tt.wantErr && safe {
				t.Error("unsafe URL was flagged as safe")
			}
		})
	}
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

		_, err := core.ParseToResponse(resp, seed)
		if err == nil {
			t.Fatal("セーフティフィルター時はエラーを返すべきなのだ")
		}
		if !strings.Contains(err.Error(), "SAFETY") {
			t.Errorf("エラーメッセージに理由が含まれていないのだ: %v", err)
		}
	})
}
