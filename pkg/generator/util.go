package generator

import (
	"fmt"
	"net"
	"net/url"
)

// IsSafeURL は、SSRF (Server-Side Request Forgery) 対策として URL を検証します。
// 許可されたスキーム (http, https) かつ、プライベートIPやループバックアドレスを
// ターゲットにしていないことを確認します。
func IsSafeURL(rawURL string) (bool, error) {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false, fmt.Errorf("URLパース失敗: %w", err)
	}

	if parsedURL.Scheme == "gs" {
		return true, nil
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

// dereferenceSeed 指定されたint64ポインタの参照解除された値を返します。ポインタがnilの場合は0を返します。
func dereferenceSeed(seed *int64) int64 {
	if seed == nil {
		return 0
	}
	return *seed
}
