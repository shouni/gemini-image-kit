package imgutil

import (
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
