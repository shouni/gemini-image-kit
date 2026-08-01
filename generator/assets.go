package generator

import (
	"bufio"
	"context"
	"fmt"
)

// EnsureUploaded は指定された fileURI の画像を Gemini File API にアップロードし、
// アップロード先の URI を返します。すでにアップロード済みならキャッシュの URI を返します。
//
// 引数の Reader を受け取る gemini.FileManager.UploadFile とは役割が異なります
// （こちらは「URL から取得してアップロードするところまで」を担います）。
func (c *GeminiImageCore) EnsureUploaded(ctx context.Context, fileURI string) (string, error) {
	if entry, ok := c.lookupCache(fileURI); ok {
		return entry.URI, nil
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
		if entry, ok := c.lookupCache(fileURI); ok {
			return entry.URI, nil
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

	c.storeCache(fileURI, cachedFile{URI: uploaded.URI, Name: uploaded.Name})
	return uploaded.URI, nil
}

// DeleteFile は指定された URI を使用して Gemini File API からファイルを削除します。
// 削除に成功した場合は、同じソース URI での再利用を防ぐためキャッシュも無効化します。
func (c *GeminiImageCore) DeleteFile(ctx context.Context, fileURI string) error {
	entry, ok := c.lookupCache(fileURI)
	if !ok || entry.Name == "" {
		return fmt.Errorf("%w: %s", ErrFileNotInCache, fileURI)
	}
	if err := c.aiClient.DeleteFile(ctx, entry.Name); err != nil {
		return err
	}
	c.removeFromCache(fileURI)
	return nil
}

// cachedFile は File API 上のファイル参照です。
//
// URI と Name を1エントリにまとめて保存します。別々のキーに分けると、
// 片方だけが失効したときに「生成には使えるが削除できない」中途半端な状態が
// 生まれるためです（DeleteFile は Name に依存します）。
type cachedFile struct {
	URI  string
	Name string
}

// lookupCache は、ソース URI に紐づくキャッシュエントリを取得します。
// 旧形式（文字列を個別キーに保存）のエントリは型アサーションに失敗して
// ミス扱いになるため、キャッシュ形式の変更は安全に無視されます。
func (c *GeminiImageCore) lookupCache(sourceURI string) (cachedFile, bool) {
	val, ok := c.cache.Get(cacheKeyFileAPI + sourceURI)
	if !ok {
		return cachedFile{}, false
	}
	entry, ok := val.(cachedFile)
	if !ok || entry.URI == "" {
		return cachedFile{}, false
	}
	return entry, true
}

// storeCache は、ソース URI に紐づくキャッシュエントリを保存します。
func (c *GeminiImageCore) storeCache(sourceURI string, entry cachedFile) {
	c.cache.Set(cacheKeyFileAPI+sourceURI, entry, c.expiration)
}

// removeFromCache は、指定されたソース URI に紐づくキャッシュエントリを削除します。
func (c *GeminiImageCore) removeFromCache(sourceURI string) {
	c.cache.Delete(cacheKeyFileAPI + sourceURI)
}
