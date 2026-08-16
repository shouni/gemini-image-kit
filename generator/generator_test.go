package generator

import (
	"context"
	"errors"
	"testing"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

// newVertexGenerator は、Vertex AI の典型構成（gs:// 直参照 + 取得フォールバック）で
// Generator を組み立てます。
func newVertexGenerator(t *testing.T, client *fakeClient) *Generator {
	t.Helper()
	client.vertexAI = true
	g, err := New(client, NewResolverChain(NewGCSResolver(), newTestFetchResolver(t, FetchResolverConfig{})))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return g
}

// TestNewRequiresClientAndResolver は、必須引数の nil チェックを検証します。
//
// resolver に既定を用意しないのは、参照の送り方（gs:// 直参照 / File API /
// インライン）が運用上の判断であり、黙って選ぶと利用側が気付けないためです。
func TestNewRequiresClientAndResolver(t *testing.T) {
	if _, err := New(nil, NewGCSResolver()); !errors.Is(err, ErrAIClientRequired) {
		t.Errorf("error = %v, want ErrAIClientRequired", err)
	}
	if _, err := New(&fakeClient{}, nil); !errors.Is(err, ErrResolverRequired) {
		t.Errorf("error = %v, want ErrResolverRequired", err)
	}
}

// TestNewRejectsVertexResolverOnGeminiAPI は、Vertex 専用の resolver に Gemini API の
// クライアントを組み合わせた取り違えを構築時に弾くことを確認します。生成時まで
// 気付けないと、参照が一切解決できない不可解な失敗になります。
func TestNewRejectsVertexResolverOnGeminiAPI(t *testing.T) {
	_, err := New(&fakeClient{vertexAI: false}, NewGCSResolver())
	if !errors.Is(err, ErrVertexAIRequired) {
		t.Fatalf("error = %v, want ErrVertexAIRequired", err)
	}

	// チェーンに含まれていても同じく弾く。
	chain := NewResolverChain(NewGCSResolver(), newTestFetchResolver(t, FetchResolverConfig{}))
	if _, err := New(&fakeClient{vertexAI: false}, chain); !errors.Is(err, ErrVertexAIRequired) {
		t.Errorf("chain error = %v, want ErrVertexAIRequired", err)
	}
}

// TestGenerateWithSingleReference は、参照 1 枚の生成が通ることを確認します。
func TestGenerateWithSingleReference(t *testing.T) {
	client := &fakeClient{}
	g := newVertexGenerator(t, client)

	resp, err := g.Generate(context.Background(), ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{
			Model:           "gemini-test-model",
			Prompt:          "test prompt",
			GenerateOptions: gemini.GenerateOptions{ImageSize: "2K"},
		},
		Images: []ports.ImageURI{{ReferenceURL: "gs://bucket/ref.png"}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !client.generateCalled {
		t.Fatal("モデルが呼ばれていません")
	}
	if resp == nil || resp.MimeType != "image/png" {
		t.Errorf("unexpected response: %+v", resp)
	}
	if got := client.attachments(); len(got) != 1 || got[0].URI != "gs://bucket/ref.png" {
		t.Errorf("attachments = %+v, want the gs:// reference", got)
	}
}

// TestGenerateWithMultipleReferences は、複数参照（融合生成）が通ることを確認します。
func TestGenerateWithMultipleReferences(t *testing.T) {
	client := &fakeClient{}
	g := newVertexGenerator(t, client)

	resp, err := g.Generate(context.Background(), ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{
			Model:           "gemini-test-model",
			Prompt:          "fuse these",
			GenerateOptions: gemini.GenerateOptions{AspectRatio: "16:9"},
		},
		Images: []ports.ImageURI{
			{ReferenceURL: "gs://bucket/one.png"},
			{ReferenceURL: "gs://bucket/two.png"},
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp == nil || resp.MimeType != "image/png" {
		t.Errorf("unexpected response: %+v", resp)
	}
	if got := client.attachments(); len(got) != 2 {
		t.Errorf("attachments = %d, want 2", len(got))
	}
}

// TestGenerateReturnsReferenceError は、参照解決の失敗でモデルを呼ばないことを
// 確認します（呼べば課金だけ発生します）。
func TestGenerateReturnsReferenceError(t *testing.T) {
	client := &fakeClient{vertexAI: true}
	fetch := newTestFetchResolver(t, FetchResolverConfig{
		Downloader: &mockHTTPClient{err: errors.New("download failed")},
	})
	g, err := New(client, NewResolverChain(NewGCSResolver(), fetch))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = g.Generate(context.Background(), ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{Model: "gemini-test-model", Prompt: "test prompt"},
		Images:            []ports.ImageURI{{ReferenceURL: "https://example.com/source.png"}},
	})
	if err == nil {
		t.Fatal("expected the reference failure")
	}
	if client.generateCalled {
		t.Fatal("参照解決に失敗したのにモデルが呼ばれています")
	}
}

func TestToOptions_PersonGeneration(t *testing.T) {
	newGenerator := func(t *testing.T, vertexAI bool) *Generator {
		t.Helper()
		client := &fakeClient{vertexAI: vertexAI}
		g, err := New(client, newTestFetchResolver(t, FetchResolverConfig{}))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		return g
	}

	t.Run("Vertex AI: 未指定なら AllowAll", func(t *testing.T) {
		g := newGenerator(t, true)
		opts := g.toOptions(ports.GenerationOptions{})
		if opts.PersonGeneration != gemini.PersonGenerationAllowAll {
			t.Errorf("PersonGeneration = %q, want %q", opts.PersonGeneration, gemini.PersonGenerationAllowAll)
		}
	})

	t.Run("Vertex AI: 指定された値を尊重する", func(t *testing.T) {
		g := newGenerator(t, true)
		opts := g.toOptions(ports.GenerationOptions{
			GenerateOptions: gemini.GenerateOptions{PersonGeneration: gemini.PersonGenerationDontAllow},
		})
		if opts.PersonGeneration != gemini.PersonGenerationDontAllow {
			t.Errorf("PersonGeneration = %q, want %q", opts.PersonGeneration, gemini.PersonGenerationDontAllow)
		}
	})

	t.Run("Gemini API: 指定されていても常に未設定", func(t *testing.T) {
		g := newGenerator(t, false)
		opts := g.toOptions(ports.GenerationOptions{
			GenerateOptions: gemini.GenerateOptions{PersonGeneration: gemini.PersonGenerationAllowAll},
		})
		if opts.PersonGeneration != gemini.PersonGenerationUnspecified {
			t.Errorf("PersonGeneration = %q, want unspecified", opts.PersonGeneration)
		}
	})
}

// backendlessClient は gemini.BackendInspector を実装しないクライアントです。
// テスト用フェイクや、判定を転送しないラッパーがこの形になります。
type backendlessClient struct{}

func (backendlessClient) GenerateWithAttachments(context.Context, string, string, []gemini.Attachment, gemini.GenerateOptions) (*gemini.Response, error) {
	return &gemini.Response{
		Attachments: []gemini.Attachment{{MIMEType: "image/png", Data: []byte("img")}},
	}, nil
}

// TestNewAllowsVertexResolverWhenBackendUndeclared は、バックエンドを申告しない
// クライアントを Vertex 専用 resolver と組み合わせても弾かないことを確認します。
//
// 「Vertex でない証拠が無い」ことを「Vertex でない」と断定すると、実クライアント
// 以外（テストダブル、判定を転送しないラッパー）で GCSResolver が一切使えなくなります。
// 弾くのは、Vertex でないと明示的に申告されたときだけです。
func TestNewAllowsVertexResolverWhenBackendUndeclared(t *testing.T) {
	g, err := New(backendlessClient{}, NewGCSResolver())
	if err != nil {
		t.Fatalf("New() error = %v, want nil (バックエンド未申告は素通し)", err)
	}
	if g.isVertexAI {
		t.Error("未申告のクライアントを Vertex と見なしてはいけません")
	}
}

// TestNewRejectsFileAPIResolverOnVertex は、Gemini API 専用の resolver に Vertex AI の
// クライアントを組み合わせた取り違えを構築時に弾くことを確認します。
//
// 弾かないと生成は「成功」してしまいます。Vertex に File API は無いのでアップロードが
// 必ず失敗し、チェーンは毎回インラインへ落ちるためです。結果は正しいのに、gs:// の
// 参照を 2 回ダウンロード（アップロード試行ぶんと取得ぶん）し、転送ゼロの経路を
// 失ったまま警告ログを出し続ける — いちばん気付きにくい失敗の仕方です。
func TestNewRejectsFileAPIResolverOnVertex(t *testing.T) {
	upload := newTestFileAPIResolver(t, FileAPIResolverConfig{})

	if _, err := New(&fakeClient{vertexAI: true}, upload); !errors.Is(err, ErrGeminiAPIRequired) {
		t.Fatalf("error = %v, want ErrGeminiAPIRequired", err)
	}

	// チェーンに含まれていても同じく弾く。
	chain := NewResolverChain(upload, newTestFetchResolver(t, FetchResolverConfig{}))
	if _, err := New(&fakeClient{vertexAI: true}, chain); !errors.Is(err, ErrGeminiAPIRequired) {
		t.Errorf("chain error = %v, want ErrGeminiAPIRequired", err)
	}

	// Gemini API のクライアントとなら通る。
	if _, err := New(&fakeClient{vertexAI: false}, chain); err != nil {
		t.Errorf("Gemini API バックエンドで error = %v, want nil", err)
	}
}
