package generator

import "errors"

var (
	// ErrFileNotInCache は、File API 上のファイル名がキャッシュから引けず削除できない場合に返されます。
	ErrFileNotInCache = errors.New("cannot determine file name for deletion, file not found in cache")
)
