package ports

import (
	"github.com/shouni/gemini-image-kit/internal/imgutil"
)

// ExtensionByMIMEType は、画像の MIMEType に対応する保存ファイルの拡張子を
// 先頭のドット付きで返します。判定できない MIMEType には ".png" を返すため、
// 保存そのものが止まることはありません。Content-Type ヘッダーの値のように
// パラメーター付きの MIMEType もそのまま渡せます。
//
// ImageResponse.MimeType をそのまま渡して、生成物の保存先を決めるためのものです。
// 標準の mime.ExtensionsByType を使わないのは、OS の MIME データベース次第で返る
// 拡張子が環境ごとに変わるためです（image/jpeg に ".jpe" が来ることもあります）。
// 保存パスは URL や履歴に残り続けるので、対応表を固定しています。
func ExtensionByMIMEType(mimeType string) string {
	return imgutil.ExtensionByMIMEType(mimeType)
}
