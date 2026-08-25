package generator

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

// collectImageAttachments は ImageURI の並びを送信用の添付へ変換します。
//
// 参照の解決は resolver 次第で GCS / HTTP の往復を伴うため並行に実行します。融合生成
// では参照が増えるほど直列の待ち時間がそのまま積み上がるためです。結果は入力順のまま
// 返します（参照画像の並び順はモデルの解釈に影響します）。resolver に課される
// 同時アクセス安全性の要求は ports.ReferenceResolver に書いてあります。
func (g *Generator) collectImageAttachments(ctx context.Context, uris []ports.ImageURI) ([]gemini.Attachment, error) {
	if len(uris) <= 1 {
		return g.collectSequentially(ctx, uris)
	}

	resolved := make([]gemini.Attachment, len(uris))
	errs := make([]error, len(uris))

	// 1つでも失敗したら残りの取得は無駄になるので打ち切る。
	fetchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i, uri := range uris {
		wg.Go(func() {
			attachment, err := g.resolveImageAttachment(fetchCtx, uri)
			if err != nil {
				errs[i] = err
				cancel()
				return
			}
			resolved[i] = attachment
		})
	}
	wg.Wait()

	if err := firstMeaningfulError(errs); err != nil {
		return nil, err
	}

	attachments := make([]gemini.Attachment, 0, len(resolved))
	for _, attachment := range resolved {
		// 参照先を持たない要素は送るものが無いので落とす。
		if !attachment.IsEmpty() {
			attachments = append(attachments, attachment)
		}
	}
	return attachments, nil
}

// collectSequentially は参照が 1 枚以下の場合の経路です。goroutine と打ち切り用の
// context を起こしても得るものが無いため、そのまま順に解決します。
func (g *Generator) collectSequentially(ctx context.Context, uris []ports.ImageURI) ([]gemini.Attachment, error) {
	attachments := make([]gemini.Attachment, 0, len(uris))
	for _, uri := range uris {
		attachment, err := g.resolveImageAttachment(ctx, uri)
		if err != nil {
			return nil, err
		}
		if !attachment.IsEmpty() {
			attachments = append(attachments, attachment)
		}
	}
	return attachments, nil
}

// firstMeaningfulError は、並行実行の結果から報告すべきエラーを 1 つ選びます。
//
// 入力順で最初のものを選ぶのは、どの参照で失敗したかが実行ごとに変わらないように
// するためです。打ち切り (context.Canceled) は最初の失敗の二次的な結果なので、
// 本来の失敗が他にあるならそちらを優先します。
func firstMeaningfulError(errs []error) error {
	for _, err := range errs {
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	// 呼び出し側の context が終了した場合など、すべてが打ち切り由来だったケース。
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// resolveImageAttachment は ImageURI 1 件を resolver に解決させます。
//
// 参照先を持たない要素は resolver へ渡さず、空の添付として落とします
// （エラーにしない理由は ports.ImageURI.IsEmpty を参照）。
func (g *Generator) resolveImageAttachment(ctx context.Context, uri ports.ImageURI) (gemini.Attachment, error) {
	if uri.IsEmpty() {
		return gemini.Attachment{}, nil
	}
	attachment, err := g.resolver.Resolve(ctx, uri)
	if err != nil {
		return gemini.Attachment{}, fmt.Errorf("failed to prepare image attachment for %q: %w", uri.ReferenceURL, err)
	}
	return attachment, nil
}
