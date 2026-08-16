package generator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

// failingExecutor は指定したプロンプトの実行だけを失敗させるスタブです。
type failingExecutor struct {
	stubExecutor
	failPrompt string
	failErr    error
}

func (f *failingExecutor) executeRequest(ctx context.Context, model, prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions) (*ports.ImageResponse, error) {
	if f.failPrompt != "" && prompt == f.failPrompt {
		return nil, f.failErr
	}
	return f.stubExecutor.executeRequest(ctx, model, prompt, attachments, opts)
}

func batchRequest(prompt string) ports.ImageRequest {
	return ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{Model: "gemini-test-model", Prompt: prompt},
	}
}

// TestGenerateBatchKeepsPartialResults は、一部の失敗が成功済みの結果を
// 破棄しないことを確認します。画像1枚ごとに生成コストが掛かっているため、
// 1件の失敗で支払い済みの成果物を捨ててはいけません。
func TestGenerateBatchKeepsPartialResults(t *testing.T) {
	boom := errors.New("generation failed")
	stub := &failingExecutor{failPrompt: "b", failErr: boom}
	g := &GeminiGenerator{core: stub, autoSeed: true, maxConcurrency: 1}

	results, err := g.GenerateBatch(context.Background(),
		[]ports.ImageRequest{batchRequest("a"), batchRequest("b"), batchRequest("c")})

	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want the generation failure", err)
	}
	if results[0] == nil || results[2] == nil {
		t.Error("成功した結果が破棄されています")
	}
	if results[1] != nil {
		t.Error("失敗した位置に結果が入っています")
	}
	if results[0] != nil && results[0].Prompt != "a" {
		t.Errorf("results[0].Prompt = %q, want request order preserved", results[0].Prompt)
	}
}

// TestGenerateBatchAllSuccessReturnsNilError は、全件成功時に error が nil で
// あることを確認します（errors.Join が nil だけを受けると nil を返す契約に乗る）。
func TestGenerateBatchAllSuccessReturnsNilError(t *testing.T) {
	stub := &stubExecutor{}
	g := newStubGenerator(stub, WithMaxConcurrency(3))

	results, err := g.GenerateBatch(context.Background(),
		[]ports.ImageRequest{batchRequest("a"), batchRequest("b")})
	if err != nil {
		t.Fatalf("GenerateBatch() error = %v", err)
	}
	for i, r := range results {
		if r == nil {
			t.Errorf("results[%d] = nil", i)
		}
	}
}

// TestGenerateBatchErrorsCarryRequestIndex は、集約エラーが何番目のリクエストで
// 起きた失敗かを示すことを確認します。添字が無いと、呼び出し側は results の nil 位置と
// 突き合わせない限り失敗したリクエストを特定できません。
func TestGenerateBatchErrorsCarryRequestIndex(t *testing.T) {
	boom := errors.New("generation failed")
	stub := &failingExecutor{failPrompt: "b", failErr: boom}
	g := &GeminiGenerator{core: stub, autoSeed: true, maxConcurrency: 1}

	_, err := g.GenerateBatch(context.Background(),
		[]ports.ImageRequest{batchRequest("a"), batchRequest("b"), batchRequest("c")})
	if err == nil {
		t.Fatal("GenerateBatch() error = nil, want the generation failure")
	}
	if !strings.Contains(err.Error(), "requests[1]:") {
		t.Errorf("error = %q, want it to name requests[1]", err)
	}
	if !errors.Is(err, boom) {
		t.Errorf("error = %v, want it to wrap the generation failure", err)
	}
}

// TestGenerateBatchFoldsNotStartedRequests は、打ち切りで開始しなかった分が
// 1 件に畳まれることを確認します。件数ぶん同じ context.Canceled を並べると、
// 大きなバッチのキャンセルでエラーが読めなくなります。
func TestGenerateBatchFoldsNotStartedRequests(t *testing.T) {
	stub := &stubExecutor{}
	g := newStubGenerator(stub)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results, err := g.GenerateBatch(ctx,
		[]ports.ImageRequest{batchRequest("a"), batchRequest("b"), batchRequest("c")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if lines := strings.Count(err.Error(), "\n") + 1; lines != 1 {
		t.Errorf("error has %d lines, want them folded into 1:\n%v", lines, err)
	}
	if !strings.Contains(err.Error(), "3 request(s) not started") {
		t.Errorf("error = %q, want the skipped count", err)
	}
	for i, r := range results {
		if r != nil {
			t.Errorf("results[%d] = %+v, want nil (not started)", i, r)
		}
	}
}

// TestGenerateValidatesBeforeRateLimit は、送信しないと決まったリクエストが
// 発射枠を待たされないことを確認します。以前は 60s 間隔の設定でモデル名が空の
// リクエストを弾くのに 60 秒掛かり、しかも発射枠を 1 つ消費していました。
func TestGenerateValidatesBeforeRateLimit(t *testing.T) {
	stub := &stubExecutor{}
	g := newStubGenerator(stub, WithRateLimit(time.Hour, 1))

	// 最初の 1 回でバーストを使い切る。以降の待機は 1 時間。
	if _, err := g.Generate(context.Background(), batchRequest("p")); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	invalid := ports.ImageRequest{GenerationOptions: ports.GenerationOptions{Prompt: "p"}}
	done := make(chan error, 1)
	go func() { _, err := g.Generate(context.Background(), invalid); done <- err }()

	select {
	case err := <-done:
		if !errors.Is(err, ErrModelRequired) {
			t.Fatalf("error = %v, want ErrModelRequired", err)
		}
	case <-time.After(time.Second):
		t.Fatal("不正なリクエストがレート制限の発射枠を待たされています")
	}
}

// TestGenerateAppliesRateLimit は、WithRateLimit が呼び出し間隔を空けることを確認します。
func TestGenerateAppliesRateLimit(t *testing.T) {
	stub := &stubExecutor{}
	g := newStubGenerator(stub, WithRateLimit(30*time.Millisecond, 1))

	start := time.Now()
	for range 3 {
		if _, err := g.Generate(context.Background(), batchRequest("p")); err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
	}
	// 1回目は即時、2・3回目がそれぞれ 30ms 待つので合計 60ms 以上掛かるはず。
	if elapsed := time.Since(start); elapsed < 55*time.Millisecond {
		t.Errorf("elapsed = %v, rate limit not applied", elapsed)
	}
}

// TestGenerateRequestTimeoutBoundsCall は、WithRequestTimeout が1回の生成を
// 打ち切ることを確認します。
func TestGenerateRequestTimeoutBoundsCall(t *testing.T) {
	slow := &slowExecutor{delay: 100 * time.Millisecond}
	g := &GeminiGenerator{core: slow, autoSeed: true, maxConcurrency: 1, requestTimeout: 5 * time.Millisecond}

	_, err := g.Generate(context.Background(), batchRequest("p"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
}

type slowExecutor struct {
	stubExecutor
	delay time.Duration
}

func (s *slowExecutor) executeRequest(ctx context.Context, model, prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions) (*ports.ImageResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(s.delay):
		return s.stubExecutor.executeRequest(ctx, model, prompt, attachments, opts)
	}
}
