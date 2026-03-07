package imgutil

import (
	"testing"
)

func TestGuessMIMEType(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"gs://bucket/image.jpg", "image/jpeg"},
		{"https://example.com/photo.JPEG", "image/jpeg"},
		{"path/to/icon.png", "image/png"},
		{"image.webp", "image/webp"},
		{"image.gif", "image/gif"},     // 新しく追加したケースにも対応
		{"document.pdf", "image/jpeg"}, // 未対応拡張子はフォールバック
		{"no_extension", "image/jpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			// パッケージ関数として直接呼び出す
			got := GuessMIMEType(tt.uri)
			if got != tt.expected {
				t.Errorf("GuessMIMEType(%q) = %q; want %q", tt.uri, got, tt.expected)
			}
		})
	}
}
