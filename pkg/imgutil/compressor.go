package imgutil

import (
	"bytes"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
)

// CompressToJPEG は画像データ（Reader）を読み込み、JPEG形式でエンコードしてバイト列を返します。
// image.Decodeがサポートするフォーマットに対応しています。
func CompressToJPEG(r io.Reader, quality int) ([]byte, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// IsCompressibleMimeType は、圧縮処理対象となるMIMEタイプを判定します。
func IsCompressibleMimeType(mimeType string) bool {
	switch mimeType {
	case "image/png", "image/gif":
		return true
	default:
		return false
	}
}
