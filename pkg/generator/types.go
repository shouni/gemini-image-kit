package generator

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
