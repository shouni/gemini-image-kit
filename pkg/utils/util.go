package utils

import (
	"github.com/shouni/go-utils/security"
)

// IsSafeURL は、SSRF対策としてURLを検証します。
func IsSafeURL(rawURL string) (bool, error) {
	return security.IsSafeURL(rawURL)
}

// DereferenceSeed は、int64のポインタを安全にデリファレンスします。
// ポインタがnilの場合は0を返します。
func DereferenceSeed(seed *int64) int64 {
	if seed == nil {
		return 0
	}
	return *seed
}
