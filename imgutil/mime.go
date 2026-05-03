package imgutil

import (
	"net/http"
	"path/filepath"
	"strings"
)

// GuessMIMEType は拡張子から MIMEType を推測します。
// 判定できない場合は "image/jpeg" を返します。
func GuessMIMEType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

// DetectMIMEType はバイト列の内容から MIMEType を判定します。
func DetectMIMEType(data []byte) string {
	return http.DetectContentType(data)
}

// IsImageMIMEType は MIMEType が画像として扱えるかを判定します。
func IsImageMIMEType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}
