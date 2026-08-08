package imgutil

import (
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// GuessMIMEType は拡張子から MIMEType を推測します。
// 判定できない場合は空文字列を返します（誤った型を申告するより、サーバー側の
// コンテンツ判定に委ねるほうが安全なため）。
//
// 署名付き URL のようなクエリ付きの URI では、クエリを除いたパス部分の拡張子を
// 見ます。素の filepath.Ext は ".png?X-Goog-Signature=..." を拡張子として返すため、
// 最も一般的な URL の形で常に判定不能になっていました。
func GuessMIMEType(rawPath string) string {
	ext := strings.ToLower(filepath.Ext(rawPath))
	if u, err := url.Parse(rawPath); err == nil && u.Path != "" {
		ext = strings.ToLower(path.Ext(u.Path))
	}
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
