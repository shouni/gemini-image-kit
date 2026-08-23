package imgutil

import (
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// MIMETypeByPath は、パスや URI の拡張子から MIMEType を引きます。
// 判定できない場合は空文字列を返します（誤った型を申告するより、サーバー側の
// コンテンツ判定に委ねるほうが安全なため）。
//
// ExtensionByMIMEType と対になる引き当てで、標準の mime.TypeByExtension /
// mime.ExtensionsByType に相当します。標準版を使わないのは、返る値が OS の MIME
// データベース次第で環境ごとに変わるためです。名前は標準側に合わせていますが、
// 受け取るのは拡張子ではなくパスや URI そのものです（クエリの扱いは下記）。
//
// 署名付き URL のようなクエリ付きの URI では、クエリを除いたパス部分の拡張子を
// 見ます。素の filepath.Ext は ".png?X-Goog-Signature=..." を拡張子として返すため、
// 最も一般的な URL の形で常に判定不能になっていました。
func MIMETypeByPath(rawPath string) string {
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

// ExtensionByMIMEType は、MIMEType に対応するファイル拡張子を先頭のドット付きで返します。
// MIMETypeByPath の逆向きの引き当てなので、対応フォーマットを増やすときは両方を直してください
// （TestMIMETypeAndExtensionRoundTrip が食い違いを検出します）。
//
// 判定できない場合は ".png" を返します。MIMETypeByPath が既定値を返さず空文字列で
// 呼び出し側に委ねるのとは逆ですが、これは両者の実害が違うためです。MIMEType の申告は
// 受け取った側の解釈をそのまま決めてしまうので、誤った申告は内容そのものを壊します。
// 一方の拡張子は保存パスの見た目に留まります。配信時の Content-Type は保存側が
// MIMEType から別途付けるため、拡張子が食い違ってもブラウザーの表示は壊れません。
// 生成済みの画像を保存できずに捨てるほうが損失は大きいので、こちらは既定へ倒します。
//
// 引数には Content-Type ヘッダーの値をそのまま渡せます。"image/jpeg; charset=binary"
// のようなパラメーター付きの値も、メディアタイプだけを見て判定します。
func ExtensionByMIMEType(mimeType string) string {
	trimmed := strings.TrimSpace(mimeType)

	// パラメーターが壊れていても mime.ParseMediaType はメディアタイプ自体を返します
	// (ErrInvalidMediaParameter)。空で返ったときだけ、生の値で判定を試みます。
	mediaType, _, err := mime.ParseMediaType(trimmed)
	if err != nil && mediaType == "" {
		mediaType = strings.ToLower(trimmed)
	}

	switch mediaType {
	case "image/jpeg", "image/jpg":
		// image/jpg は正式な MIMEType ではありませんが、生成モデルの応答や手書きの
		// 設定値では実際に現れます。既定の .png へ倒すと中身と拡張子が食い違うため、
		// 綴りの揺れとして扱います。MIMETypeByPath が ".jpg" と ".jpeg" の
		// どちらも受けるのと同じ理由です。
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
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
