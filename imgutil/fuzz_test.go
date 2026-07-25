package imgutil

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// validPNG は Fuzz のシードに使う最小の正当な PNG を返します。
func validPNG(tb testing.TB) []byte {
	tb.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		tb.Fatalf("シード PNG の生成に失敗しました: %v", err)
	}
	return buf.Bytes()
}

// FuzzDetectMIMEType は、任意のバイト列で panic せず、
// 画像と判定した場合は必ず既知の image/* を返すことを検証します。
//
// この関数の戻り値はアップロード可否の判定に直結するため、
// 未知の値を「画像」と誤判定すると壊れたデータがそのまま送信されます。
func FuzzDetectMIMEType(f *testing.F) {
	f.Add(validPNG(f))
	f.Add([]byte("\xff\xd8\xff\xe0"))             // JPEG ヘッダ
	f.Add([]byte("RIFF\x00\x00\x00\x00WEBPVP8 ")) // WebP ヘッダ
	f.Add([]byte("GIF89a"))
	f.Add([]byte("%PDF-1.4"))
	f.Add([]byte(""))
	f.Add([]byte("\x00"))
	f.Add(bytes.Repeat([]byte{0xff}, 1024))

	f.Fuzz(func(t *testing.T, data []byte) {
		mimeType := DetectMIMEType(data)

		if !IsImageMIMEType(mimeType) {
			return
		}

		// 画像と判定した以上、image/ 接頭辞を持つ具体的な型でなければならない。
		if !strings.HasPrefix(mimeType, "image/") {
			t.Fatalf("画像と判定されたのに image/ で始まりません: %q", mimeType)
		}

		// 圧縮対象と判定したものは、圧縮関数に渡しても panic してはならない。
		if IsCompressibleMimeType(mimeType) {
			_, _ = CompressToJPEG(bytes.NewReader(data), 75)
		}
	})
}

// FuzzCompressToJPEG は、壊れた画像データを渡しても panic せず、
// 成功した場合は必ずデコード可能な JPEG を返すことを検証します。
func FuzzCompressToJPEG(f *testing.F) {
	f.Add(validPNG(f), 75)
	f.Add([]byte("not an image"), 75)
	f.Add([]byte(""), 0)
	f.Add(validPNG(f), 1)
	f.Add(validPNG(f), 100)
	f.Add(validPNG(f), -1)
	f.Add(validPNG(f), 999)

	f.Fuzz(func(t *testing.T, data []byte, quality int) {
		out, err := CompressToJPEG(bytes.NewReader(data), quality)
		if err != nil {
			return
		}

		// エラーを返さなかった以上、戻り値は妥当な画像でなければならない。
		// そうでないと壊れたデータが検証をすり抜けてアップロードされる。
		if len(out) == 0 {
			t.Fatal("エラーなしで空のデータが返りました")
		}
		if _, _, err := image.Decode(bytes.NewReader(out)); err != nil {
			t.Fatalf("圧縮結果がデコードできません (quality=%d): %v", quality, err)
		}
	})
}
