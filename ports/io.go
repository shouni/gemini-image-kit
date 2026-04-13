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
	// Write は、指定された path に応じて GCS、S3、またはローカルファイルへデータを書き込みます。
	Write(ctx context.Context, path string, contentReader io.Reader, contentType string) error
}
