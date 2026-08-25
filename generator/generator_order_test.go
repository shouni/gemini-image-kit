package generator

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

// TestGenerateValidatesBeforeResolvingReferences は、送信しないと決まった
// リクエストのために参照画像の解決（取得・アップロード）が走らないことを
// 確認します。prepare を後ろに回すと、捨てる結果のために I/O を払うことになります。
func TestGenerateValidatesBeforeResolvingReferences(t *testing.T) {
	var resolved atomic.Int64
	resolver := &stubResolver{
		resolve: func(_ context.Context, rawURL string) (gemini.Attachment, error) {
			resolved.Add(1)
			return gemini.Attachment{MIMEType: "image/png", Data: []byte(rawURL)}, nil
		},
	}
	g, client := newStubGenerator(t, resolver)

	// Model が空なので prepare で弾かれる。参照画像は付いている。
	_, err := g.Generate(context.Background(), ports.ImageRequest{
		Prompt: "p",
		Images: []ports.ImageURI{{ReferenceURL: "gs://bucket/ref.png"}},
	})

	if !errors.Is(err, ErrModelRequired) {
		t.Fatalf("error = %v, want ErrModelRequired", err)
	}
	if n := resolved.Load(); n != 0 {
		t.Errorf("参照画像の解決が %d 回走っています。不正なリクエストに I/O を払ってはいけません", n)
	}
	if client.generateCalled {
		t.Error("モデル呼び出しが行われています")
	}
}
