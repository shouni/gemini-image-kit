package ports

import (
	"context"
	"io"
)

// ContentReader は、指定されたURIからコンテンツを取得するためのインターフェースです。
type ContentReader interface {
	Open(ctx context.Context, uri string) (io.ReadCloser, error)
}

// Downloader は URL からデータストリームを取得するためのインターフェイスです。
type Downloader interface {
	// GetStream は指定された URL からデータストリームを取得します。
	// 呼び出し元は、使用後に必ず戻り値の io.ReadCloser を Close() する責任があります。
	GetStream(ctx context.Context, url string) (io.ReadCloser, error)
}
