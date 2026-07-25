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
		// 推測できない場合に既定値を返すと、PNG を JPEG と申告するような
		// 誤った型宣言になりうる。空文字列を返して呼び出し側に判断を委ね、
		// サーバー側のコンテンツ判定に任せる。
		return ""
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
