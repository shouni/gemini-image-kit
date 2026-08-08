package generator

import (
	"bufio"
	"context"
)

// ensureUploaded は指定された fileURI の画像を Gemini File API にアップロードし、
// アップロード先の URI を返します。すでにアップロード済みならキャッシュの URI を返します。
//
// 引数の Reader を受け取る gemini.FileManager.UploadFile とは役割が異なります
// （こちらは「URL から取得してアップロードするところまで」を担います）。
func (c *GeminiImageCore) ensureUploaded(ctx context.Context, fileURI string) (string, error) {
	if uri, ok := c.lookupCache(fileURI); ok {
		return uri, nil
	}
	return c.uploadOnce(ctx, fileURI)
}

// uploadOnce は、同一ソースへの同時アップロードを1回にまとめます。
//
// キャッシュは完了後にしか書かれないため、これが無いと同じ参照画像を並行して
// 使う呼び出し（複数参照の生成や、同じキャラクターを使う複数ジョブ）がそれぞれ
// アップロードし、File API 上に重複ファイルを作ります。
//
// 共有実行は呼び出し元の context から切り離します。最初の呼び出し元がキャンセルした
// だけで、相乗りしている他の呼び出し元まで巻き添えになるのを避けるためです。
// 打ち切りは uploadTimeout が担います。
func (c *GeminiImageCore) uploadOnce(ctx context.Context, fileURI string) (string, error) {
	ch := c.uploadGroup.DoChan(fileURI, func() (any, error) {
		execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.uploadTimeout)
		defer cancel()

		// 直前に別の実行が完了している可能性があるため、もう一度キャッシュを見る。
		if uri, ok := c.lookupCache(fileURI); ok {
			return uri, nil
		}
		return c.fetchAndUpload(execCtx, fileURI)
	})

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-ch:
		if result.Err != nil {
			return "", result.Err
		}
		uri, _ := result.Val.(string)
		return uri, nil
	}
}

// fetchAndUpload はソースを取得して File API へアップロードし、結果をキャッシュします。
func (c *GeminiImageCore) fetchAndUpload(ctx context.Context, fileURI string) (string, error) {
	rc, err := c.fetchImageData(ctx, fileURI)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	br := bufio.NewReader(rc)
	mimeType, err := detectUploadSource(br)
	if err != nil {
		return "", err
	}

	uploaded, err := c.uploadByStrategy(ctx, br, mimeType, fileURI)
	if err != nil {
		return "", err
	}

	c.storeCache(fileURI, uploaded.URI)
	return uploaded.URI, nil
}

// lookupCache は、ソース URI に紐づくアップロード済み URI を取得します。
// 旧形式（構造体を保存）のエントリは型アサーションに失敗してミス扱いになるため、
// キャッシュ形式の変更は安全に無視されます。
func (c *GeminiImageCore) lookupCache(sourceURI string) (string, bool) {
	val, ok := c.cache.Get(cacheKeyFileAPI + sourceURI)
	if !ok {
		return "", false
	}
	uri, ok := val.(string)
	if !ok || uri == "" {
		return "", false
	}
	return uri, true
}

// storeCache は、ソース URI に紐づくアップロード済み URI を保存します。
func (c *GeminiImageCore) storeCache(sourceURI string, uploadedURI string) {
	c.cache.Set(cacheKeyFileAPI+sourceURI, uploadedURI, c.expiration)
}
