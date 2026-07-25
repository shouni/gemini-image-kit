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
		{"image.gif", "image/gif"}, // 新しく追加したケースにも対応
		{"document.pdf", ""},       // 判別できない場合は空。誤った型を申告しない
		{"no_extension", ""},
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
