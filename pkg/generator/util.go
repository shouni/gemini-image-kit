package generator

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
