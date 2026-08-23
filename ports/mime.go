package ports

import (
	"github.com/shouni/gemini-image-kit/internal/imgutil"
)

// ExtensionByMIMEType は、画像の MIMEType に対応する保存ファイルの拡張子を
// 先頭のドット付きで返します。判定できない MIMEType には ".png" を返します。
//
// ImageResponse.MimeType をそのまま渡して、生成物の保存先を決めるためのものです。
// 標準の mime.ExtensionsByType は OS の MIME データベースを引くため返る拡張子が
// 環境で変わり（image/jpeg に ".jpe" が来ることもあります）、保存パスは URL や
// 履歴に残り続けるので、対応表を固定してここに置きます。
//
// 実体は internal/imgutil にあり、拡張子から MIMEType を引く逆向きの対応表と
// 同じファイルで並んでいます。片方だけフォーマットが増えると往復が壊れるためです。
func ExtensionByMIMEType(mimeType string) string {
	return imgutil.ExtensionByMIMEType(mimeType)
}
