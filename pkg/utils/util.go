package utils

// DereferenceSeed は、int64のポインタを安全にデリファレンスします。
// ポインタがnilの場合は0を返します。
func DereferenceSeed(seed *int64) int64 {
	if seed == nil {
		return 0
	}
	return *seed
}
