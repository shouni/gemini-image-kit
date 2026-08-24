package generator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

func newFusionRequest(urls ...string) ports.ImageRequest {
	images := make([]ports.ImageURI, 0, len(urls))
	for _, url := range urls {
		images = append(images, ports.ImageURI{ReferenceURL: url})
	}
	return ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{Model: "gemini-test-model", Prompt: "fuse"},
		Images:            images,
	}
}

// TestCollectImageAttachmentsPreservesOrder は、並行取得しても参照画像の並びが
// 入力順のままであることを確認します。順序はモデルの解釈に影響するため、
// 取得完了順に並べ替わってはいけません。
func TestCollectImageAttachmentsPreservesOrder(t *testing.T) {
	// 取得の遅延は仮想時間で消化されます。完了順のずれは保たれたまま実時間は消費しません。
	synctest.Test(t, func(t *testing.T) {
		// 後ろの画像ほど速く返るようにして、完了順と入力順をずらす。
		delays := map[string]time.Duration{"a": 30 * time.Millisecond, "b": 15 * time.Millisecond, "c": 0}
		resolver := &stubResolver{
			resolve: func(_ context.Context, rawURL string) (gemini.Attachment, error) {
				time.Sleep(delays[rawURL])
				return gemini.Attachment{MIMEType: "image/png", Data: []byte(rawURL)}, nil
			},
		}
		g, client := newStubGenerator(t, resolver)

		if _, err := g.Generate(context.Background(), newFusionRequest("a", "b", "c")); err != nil {
			t.Fatalf("Generate() error = %v", err)
		}

		attachments := client.attachments()
		got := make([]string, 0, len(attachments))
		for _, attachment := range attachments {
			got = append(got, string(attachment.Data))
		}
		want := []string{"a", "b", "c"}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("attachments = %v, want %v", got, want)
		}
	})
}

// TestCollectImageAttachmentsRunsConcurrently は、参照画像の取得が並行に走ることを
// 確認します。全件が揃うまで解放されないバリアを使うため、逐次実装ならタイムアウトします。
func TestCollectImageAttachmentsRunsConcurrently(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const refs = 3
		var (
			mu      sync.Mutex
			entered int
			barrier = make(chan struct{})
		)
		resolver := &stubResolver{
			resolve: func(_ context.Context, rawURL string) (gemini.Attachment, error) {
				mu.Lock()
				entered++
				reached := entered == refs
				mu.Unlock()
				if reached {
					close(barrier)
				}
				select {
				case <-barrier:
					return gemini.Attachment{MIMEType: "image/png", Data: []byte(rawURL)}, nil
				case <-time.After(2 * time.Second):
					return gemini.Attachment{}, errors.New("timed out waiting for concurrent fetches")
				}
			},
		}
		g, client := newStubGenerator(t, resolver)

		if _, err := g.Generate(context.Background(), newFusionRequest("a", "b", "c")); err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if got := len(client.attachments()); got != refs {
			t.Errorf("attachments = %d, want %d", got, refs)
		}
	})
}

// TestCollectImageAttachmentsReportsFirstErrorByIndex は、複数が失敗しても報告される
// エラーが入力順で最初のものに固定されることを確認します。並行実行では完了順が
// 実行ごとに変わるため、そのまま返すとエラーが非決定になります。
func TestCollectImageAttachmentsReportsFirstErrorByIndex(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		first := errors.New("first reference failed")
		second := errors.New("second reference failed")
		resolver := &stubResolver{
			resolve: func(_ context.Context, rawURL string) (gemini.Attachment, error) {
				switch rawURL {
				case "b":
					time.Sleep(20 * time.Millisecond) // 後から失敗させる
					return gemini.Attachment{}, first
				case "c":
					return gemini.Attachment{}, second
				default:
					return gemini.Attachment{MIMEType: "image/png", Data: []byte(rawURL)}, nil
				}
			},
		}
		g, _ := newStubGenerator(t, resolver)

		_, err := g.Generate(context.Background(), newFusionRequest("a", "b", "c"))
		if !errors.Is(err, first) {
			t.Errorf("error = %v, want the error from the first failing reference (%v)", err, first)
		}
	})
}

// TestCollectImageAttachmentsPropagatesCancellation は、呼び出し側の context 終了が
// 打ち切り理由としてそのまま返ることを確認します。
func TestCollectImageAttachmentsPropagatesCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		resolver := &stubResolver{
			resolve: func(ctx context.Context, _ string) (gemini.Attachment, error) {
				<-ctx.Done()
				return gemini.Attachment{}, ctx.Err()
			},
		}
		g, _ := newStubGenerator(t, resolver)

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			_, err := g.Generate(ctx, newFusionRequest("a", "b"))
			errCh <- err
		}()

		// 取得が ctx 待ちに入ってからキャンセルする（待ち時間の見積もりに頼らない）。
		synctest.Wait()
		cancel()

		if err := <-errCh; !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want context.Canceled", err)
		}
	})
}

// TestCollectImageAttachmentsSkipsEmpty は、参照先を持たない要素が resolver へ
// 渡らず、送信からも落ちることを確認します。テキストのみの生成や「このキャラクターには
// 参照画像が無い」を表現するため、エラーにはしません。
func TestCollectImageAttachmentsSkipsEmpty(t *testing.T) {
	called := false
	resolver := &stubResolver{resolve: func(context.Context, string) (gemini.Attachment, error) {
		called = true
		return gemini.Attachment{MIMEType: "image/png", Data: []byte("x")}, nil
	}}
	g, client := newStubGenerator(t, resolver)

	_, err := g.Generate(context.Background(), ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{Model: "gemini-test-model", Prompt: "p"},
		Images:            []ports.ImageURI{{}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if called {
		t.Error("空の参照が resolver へ渡っています")
	}
	if got := len(client.attachments()); got != 0 {
		t.Errorf("attachments = %d, want 0", got)
	}
}

// TestAutoSeedFillsMissingSeedByDefault は、シード未指定の生成でも既定で UsedSeed が
// 実際に送ったシードを指すことを確認します。0 のままだと「同条件での再生成」の記録が
// 嘘になります（以前はオプトインで、有効にしていない下流が誤記録していました）。
func TestAutoSeedFillsMissingSeedByDefault(t *testing.T) {
	g, client := newStubGenerator(t, &stubResolver{})

	resp, err := g.Generate(context.Background(), ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{Model: "gemini-test-model", Prompt: "a cat"},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	sent := client.options().Seed
	if sent == nil {
		t.Fatal("Seed was not sent to the API")
	}
	if resp.UsedSeed != *sent {
		t.Errorf("UsedSeed = %d, want the seed actually sent (%d)", resp.UsedSeed, *sent)
	}
	if resp.UsedSeed <= 0 {
		t.Errorf("UsedSeed = %d, want a usable seed", resp.UsedSeed)
	}
}

// TestAutoSeedKeepsExplicitSeed は、明示されたシードを上書きしないことを確認します。
func TestAutoSeedKeepsExplicitSeed(t *testing.T) {
	g, _ := newStubGenerator(t, &stubResolver{})

	seed := int64(4242)
	resp, err := g.Generate(context.Background(), ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{
			Model: "gemini-test-model", Prompt: "a cat",
			GenerateOptions: gemini.GenerateOptions{Seed: &seed},
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.UsedSeed != seed {
		t.Errorf("UsedSeed = %d, want %d", resp.UsedSeed, seed)
	}
}

// TestWithoutAutoSeedLeavesSeedUnset は、WithoutAutoSeed 指定時はシードを送らない
// ことを確認します（API 側がランダムに選び、UsedSeed は 0 のまま）。
func TestWithoutAutoSeedLeavesSeedUnset(t *testing.T) {
	g, client := newStubGenerator(t, &stubResolver{}, WithoutAutoSeed())

	if _, err := g.Generate(context.Background(), ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{Model: "gemini-test-model", Prompt: "a cat"},
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if sent := client.options().Seed; sent != nil {
		t.Errorf("Seed = %d, want no seed with WithoutAutoSeed", *sent)
	}
}
