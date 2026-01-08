package imgutil

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// テスト用のダミー画像（10x10の赤い正方形）を作成するヘルパー
func createDummyImageData(t *testing.T, format string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	buf := new(bytes.Buffer)
	var err error
	switch format {
	case "png":
		err = png.Encode(buf, img)
	case "jpeg":
		err = jpeg.Encode(buf, img, nil)
	default:
		t.Fatalf("unsupported format: %s", format)
	}

	if err != nil {
		t.Fatalf("failed to encode dummy image: %v", err)
	}
	return buf.Bytes()
}

func TestCompressToJPEG(t *testing.T) {
	t.Run("正常なPNG画像をJPEGに圧縮できること", func(t *testing.T) {
		pngData := createDummyImageData(t, "png")

		got, err := CompressToJPEG(pngData, 75)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(got) == 0 {
			t.Error("expected output data, but got empty")
		}

		// 出力がJPEGとしてデコード可能か確認
		_, format, err := image.Decode(bytes.NewReader(got))
		if err != nil {
			t.Errorf("failed to decode output image: %v", err)
		}
		if format != "jpeg" {
			t.Errorf("expected format jpeg, got %s", format)
		}
	})

	t.Run("不正なデータを与えた場合にエラーを返すこと", func(t *testing.T) {
		invalidData := []byte("this is not an image")
		_, err := CompressToJPEG(invalidData, 75)
		if err == nil {
			t.Error("expected error for invalid data, but got nil")
		}
	})

	t.Run("Quality設定によってサイズが変化すること", func(t *testing.T) {
		input := createDummyImageData(t, "png")

		highQuality, _ := CompressToJPEG(input, 100)
		lowQuality, _ := CompressToJPEG(input, 10)

		if len(lowQuality) >= len(highQuality) {
			t.Errorf("low quality size (%d) should be smaller than high quality size (%d)", len(lowQuality), len(highQuality))
		}
	})
}
