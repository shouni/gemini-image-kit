package generator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

func newFusionRequest(urls ...string) ports.ImageFusionRequest {
	images := make([]ports.ImageURI, 0, len(urls))
	for _, url := range urls {
		images = append(images, ports.ImageURI{ReferenceURL: url})
	}
	return ports.ImageFusionRequest{
		GenerationOptions: ports.GenerationOptions{Model: "gemini-test-model", Prompt: "fuse"},
		Images:            images,
	}
}

// TestCollectImageAttachmentsPreservesOrder は、並行取得しても参照画像の並びが
// 入力順のままであることを確認します。順序はモデルの解釈に影響するため、
// 取得完了順に並べ替わってはいけません。
func TestCollectImageAttachmentsPreservesOrder(t *testing.T) {
	// 後ろの画像ほど速く返るようにして、完了順と入力順をずらす。
	delays := map[string]time.Duration{"a": 30 * time.Millisecond, "b": 15 * time.Millisecond, "c": 0}
	stub := &stubExecutor{
		prepare: func(_ context.Context, rawURL string) (gemini.Attachment, error) {
			time.Sleep(delays[rawURL])
			return gemini.Attachment{MIMEType: "image/png", Data: []byte(rawURL)}, nil
		},
	}
	g, err := NewGeminiGenerator(stub)
	if err != nil {
		t.Fatalf("NewGeminiGenerator() error = %v", err)
	}

	if _, err := g.GenerateFusedImage(context.Background(), newFusionRequest("a", "b", "c")); err != nil {
		t.Fatalf("GenerateFusedImage() error = %v", err)
	}

	got := make([]string, 0, len(stub.lastAttachments))
	for _, attachment := range stub.lastAttachments {
		got = append(got, string(attachment.Data))
	}
	want := []string{"a", "b", "c"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("attachments = %v, want %v", got, want)
	}
}

// TestCollectImageAttachmentsRunsConcurrently は、参照画像の取得が並行に走ることを
// 確認します。全件が揃うまで解放されないバリアを使うため、逐次実装ならタイムアウトします。
func TestCollectImageAttachmentsRunsConcurrently(t *testing.T) {
	const refs = 3
	var (
		mu      sync.Mutex
		entered int
		barrier = make(chan struct{})
	)
	stub := &stubExecutor{
		prepare: func(_ context.Context, rawURL string) (gemini.Attachment, error) {
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
	g, err := NewGeminiGenerator(stub)
	if err != nil {
		t.Fatalf("NewGeminiGenerator() error = %v", err)
	}

	if _, err := g.GenerateFusedImage(context.Background(), newFusionRequest("a", "b", "c")); err != nil {
		t.Fatalf("GenerateFusedImage() error = %v", err)
	}
	if len(stub.lastAttachments) != refs {
		t.Errorf("attachments = %d, want %d", len(stub.lastAttachments), refs)
	}
}

// TestCollectImageAttachmentsReportsFirstErrorByIndex は、複数が失敗しても報告される
// エラーが入力順で最初のものに固定されることを確認します。並行実行では完了順が
// 実行ごとに変わるため、そのまま返すとエラーが非決定になります。
func TestCollectImageAttachmentsReportsFirstErrorByIndex(t *testing.T) {
	first := errors.New("first reference failed")
	second := errors.New("second reference failed")
	stub := &stubExecutor{
		prepare: func(_ context.Context, rawURL string) (gemini.Attachment, error) {
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
	g, err := NewGeminiGenerator(stub)
	if err != nil {
		t.Fatalf("NewGeminiGenerator() error = %v", err)
	}

	_, err = g.GenerateFusedImage(context.Background(), newFusionRequest("a", "b", "c"))
	if !errors.Is(err, first) {
		t.Errorf("error = %v, want the error from the first failing reference (%v)", err, first)
	}
}

// TestCollectImageAttachmentsPropagatesCancellation は、呼び出し側の context 終了が
// 打ち切り理由としてそのまま返ることを確認します。
func TestCollectImageAttachmentsPropagatesCancellation(t *testing.T) {
	stub := &stubExecutor{
		prepare: func(ctx context.Context, _ string) (gemini.Attachment, error) {
			<-ctx.Done()
			return gemini.Attachment{}, ctx.Err()
		},
	}
	g, err := NewGeminiGenerator(stub)
	if err != nil {
		t.Fatalf("NewGeminiGenerator() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err = g.GenerateFusedImage(ctx, newFusionRequest("a", "b"))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

// TestWithAutoSeedFillsMissingSeed は、シード未指定の生成でも UsedSeed が実際に
// 送ったシードを指すことを確認します。0 のままだと「同条件での再生成」の記録が嘘になります。
func TestWithAutoSeedFillsMissingSeed(t *testing.T) {
	stub := &stubExecutor{}
	g, err := NewGeminiGenerator(stub, WithAutoSeed())
	if err != nil {
		t.Fatalf("NewGeminiGenerator() error = %v", err)
	}

	resp, err := g.GenerateSingleImage(context.Background(), ports.SingleImageRequest{
		GenerationOptions: ports.GenerationOptions{Model: "gemini-test-model", Prompt: "a cat"},
	})
	if err != nil {
		t.Fatalf("GenerateSingleImage() error = %v", err)
	}

	if stub.lastOptions.Seed == nil {
		t.Fatal("Seed was not sent to the API")
	}
	if resp.UsedSeed != *stub.lastOptions.Seed {
		t.Errorf("UsedSeed = %d, want the seed actually sent (%d)", resp.UsedSeed, *stub.lastOptions.Seed)
	}
	if resp.UsedSeed <= 0 {
		t.Errorf("UsedSeed = %d, want a usable seed", resp.UsedSeed)
	}
}

// TestWithAutoSeedKeepsExplicitSeed は、明示されたシードを上書きしないことを確認します。
func TestWithAutoSeedKeepsExplicitSeed(t *testing.T) {
	stub := &stubExecutor{}
	g, err := NewGeminiGenerator(stub, WithAutoSeed())
	if err != nil {
		t.Fatalf("NewGeminiGenerator() error = %v", err)
	}

	seed := int64(4242)
	resp, err := g.GenerateSingleImage(context.Background(), ports.SingleImageRequest{
		GenerationOptions: ports.GenerationOptions{Model: "gemini-test-model", Prompt: "a cat", Seed: &seed},
	})
	if err != nil {
		t.Fatalf("GenerateSingleImage() error = %v", err)
	}
	if resp.UsedSeed != seed {
		t.Errorf("UsedSeed = %d, want %d", resp.UsedSeed, seed)
	}
}

// TestWithoutAutoSeedLeavesSeedUnset は、既定では従来どおりシードを送らないことを
// 確認します（API 側がランダムに選び、UsedSeed は 0 のまま）。
func TestWithoutAutoSeedLeavesSeedUnset(t *testing.T) {
	stub := &stubExecutor{}
	g, err := NewGeminiGenerator(stub)
	if err != nil {
		t.Fatalf("NewGeminiGenerator() error = %v", err)
	}

	if _, err := g.GenerateSingleImage(context.Background(), ports.SingleImageRequest{
		GenerationOptions: ports.GenerationOptions{Model: "gemini-test-model", Prompt: "a cat"},
	}); err != nil {
		t.Fatalf("GenerateSingleImage() error = %v", err)
	}
	if stub.lastOptions.Seed != nil {
		t.Errorf("Seed = %d, want no seed without WithAutoSeed", *stub.lastOptions.Seed)
	}
}
