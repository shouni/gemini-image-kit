package ports

import "testing"

// ExtensionByMIMEType は internal/imgutil への委譲なので、対応表そのものの網羅は
// imgutil 側のテストが持ちます。ここでは公開の入り口が繋がっていることだけを確認します。
func TestExtensionByMIMEType(t *testing.T) {
	resp := ImageResponse{MimeType: "image/webp"}

	if got := ExtensionByMIMEType(resp.MimeType); got != ".webp" {
		t.Errorf("ExtensionByMIMEType(%q) = %q; want .webp", resp.MimeType, got)
	}
	if got := ExtensionByMIMEType("image/avif"); got != ".png" {
		t.Errorf("ExtensionByMIMEType(unknown) = %q; want .png", got)
	}
}
