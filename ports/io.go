package ports

import (
	"context"
	"io"
)

// ContentReader は、指定されたURIからコンテンツを取得するためのインターフェースです。
type ContentReader interface {
	Open(ctx context.Context, uri string) (io.ReadCloser, error)
}

// ContentWriter はコンテンツを書き込むためのインターフェースです。
type ContentWriter interface {
	// Write は、指定された path または URI へデータを書き込みます。
	Write(ctx context.Context, path string, contentReader io.Reader, contentType string) error
}

// Downloader は URL からデータをダウンロードし、提供された関数を使用してデータ ストリームを処理するためのインターフェイスを定義します。
type Downloader interface {
	FetchStream(ctx context.Context, url string, fn func(io.Reader) error) error
	GetStream(ctx context.Context, url string) (io.ReadCloser, error)
}
