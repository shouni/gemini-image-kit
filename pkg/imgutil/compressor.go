package imgutil

import (
	"bytes"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
)

// CompressToJPEG は画像データ（PNG, GIF, JPEG等）をJPEG形式に圧縮します。
// image.Decodeがサポートするフォーマットに対応しています。
func CompressToJPEG(data []byte, quality int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
