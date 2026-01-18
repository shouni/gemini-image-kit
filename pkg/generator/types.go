package generator

import (
	"context"
	"time"
)

const (
	UseImageCompression     = true
	ImageCompressionQuality = 75
	cacheKeyFileAPIURI      = "fileapi_uri:"
	cacheKeyFileAPIName     = "fileapi_name:"
)

// ImageOutput は Core の内部解析結果
type ImageOutput struct {
	Data     []byte
	MimeType string
	UsedSeed int64
}

// 依存関係用
type HTTPClient interface {
	FetchBytes(ctx context.Context, url string) ([]byte, error)
}

type ImageCacher interface {
	Get(key string) (any, bool)
	Set(key string, value any, d time.Duration)
}
