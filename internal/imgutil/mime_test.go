package imgutil

import (
	"testing"
)

func TestMIMETypeByPath(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"gs://bucket/image.jpg", "image/jpeg"},
		{"https://example.com/photo.JPEG", "image/jpeg"},
		{"path/to/icon.png", "image/png"},
		{"image.webp", "image/webp"},
		{"image.gif", "image/gif"}, // 新しく追加したケースにも対応
		{"document.pdf", ""},       // 判別できない場合は空。誤った型を申告しない
		{"no_extension", ""},
		// 署名付き URL: クエリを除いたパスの拡張子で判定する。
		// 素の filepath.Ext だと ".png?X-Goog-Signature=abc" となり常に判定不能だった。
		{"https://storage.googleapis.com/bucket/ref.png?X-Goog-Signature=abc&X-Goog-Expires=3600", "image/png"},
		{"https://example.com/photo.jpg?w=1200", "image/jpeg"},
		{"https://example.com/download?id=123", ""},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			// パッケージ関数として直接呼び出す
			got := MIMETypeByPath(tt.uri)
			if got != tt.expected {
				t.Errorf("MIMETypeByPath(%q) = %q; want %q", tt.uri, got, tt.expected)
			}
		})
	}
}

func TestExtensionByMIMEType(t *testing.T) {
	tests := []struct {
		mimeType string
		expected string
	}{
		{"image/png", ".png"},
		{"image/jpeg", ".jpg"},
		{"image/webp", ".webp"},
		{"image/gif", ".gif"},
		// 正式な MIMEType ではないが、生成モデルの応答や手書きの設定値に現れる
		{"image/jpg", ".jpg"},
		{"IMAGE/JPEG", ".jpg"},
		{"  image/webp  ", ".webp"},
		// Content-Type ヘッダーの値をそのまま渡せる
		{"image/jpeg; charset=binary", ".jpg"},
		{"image/gif; charset", ".gif"}, // パラメーターが壊れていてもメディアタイプで判定
		// 判定できない場合は既定へ倒す。誤った拡張子の実害は保存パスの見た目に留まり、
		// 保存を止めるほうが損失が大きい（MIMETypeByPath が空を返すのとは逆の判断）
		{"image/avif", ".png"},
		{"application/json", ".png"},
		{"", ".png"},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			got := ExtensionByMIMEType(tt.mimeType)
			if got != tt.expected {
				t.Errorf("ExtensionByMIMEType(%q) = %q; want %q", tt.mimeType, got, tt.expected)
			}
		})
	}
}

// MIMETypeByPath と ExtensionByMIMEType は互いの逆向きの対応表なので、往復して元に戻ること。
// 対応フォーマットを片方にだけ足すと、ここで食い違いが出ます。
func TestMIMETypeAndExtensionRoundTrip(t *testing.T) {
	for _, mimeType := range []string{"image/png", "image/jpeg", "image/webp", "image/gif"} {
		t.Run(mimeType, func(t *testing.T) {
			ext := ExtensionByMIMEType(mimeType)

			if got := MIMETypeByPath("image" + ext); got != mimeType {
				t.Errorf("MIMETypeByPath(%q) = %q; want %q （対応表が片方だけ更新されています）", "image"+ext, got, mimeType)
			}
		})
	}
}

func TestDetectMIMEType(t *testing.T) {
	pngHeader := []byte("\x89PNG\r\n\x1a\n")

	got := DetectMIMEType(pngHeader)

	if got != "image/png" {
		t.Errorf("DetectMIMEType() = %q; want image/png", got)
	}
}

func TestIsImageMIMEType(t *testing.T) {
	tests := []struct {
		mimeType string
		expected bool
	}{
		{"image/png", true},
		{"image/jpeg", true},
		{"text/plain; charset=utf-8", false},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			got := IsImageMIMEType(tt.mimeType)
			if got != tt.expected {
				t.Errorf("IsImageMIMEType(%q) = %v; want %v", tt.mimeType, got, tt.expected)
			}
		})
	}
}
