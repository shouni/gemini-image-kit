package generator

import (
	"fmt"
	"net"
	"net/url"
)

// seedToPtrInt32 は domain の *int64 を SDK 用の *int32 に変換するのだ。
// Imagen API は int32 を期待しているための調整なのだ。
func seedToPtrInt32(s *int64) *int32 {
	if s == nil {
		return nil
	}
	v := int32(*s)
	return &v
}

// dereferenceSeed は *int64 を安全に int64 に変換するのだ。
// nil の場合はデフォルト値（0）を返すのだよ。
func dereferenceSeed(s *int64) int64 {
	if s == nil {
		return 0
	}
	return *s
}

// IsSafeURL は、SSRF (Server-Side Request Forgery) 対策として URL を検証します。
// 許可されたスキーム (http, https) かつ、プライベートIPやループバックアドレスを
// ターゲットにしていないことを確認します。
func IsSafeURL(rawURL string) (bool, error) {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false, fmt.Errorf("URLパース失敗: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false, fmt.Errorf("不許可スキーム: %s", parsedURL.Scheme)
	}

	ips, err := net.LookupIP(parsedURL.Hostname())
	if err != nil {
		return false, fmt.Errorf("ホスト '%s' の名前解決に失敗しました: %w", parsedURL.Hostname(), err)
	}

	for _, ip := range ips {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return false, fmt.Errorf("制限されたネットワークへのアクセスを検知: %s", ip.String())
		}
	}

	return true, nil
}
