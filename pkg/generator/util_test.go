package generator

import "testing"

func TestSeedUtils(t *testing.T) {
	t.Run("seedToPtrInt32: nil の場合は nil を返すのだ", func(t *testing.T) {
		if got := seedToPtrInt32(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("seedToPtrInt32: 値がある場合は int32 に変換されるのだ", func(t *testing.T) {
		var val int64 = 12345
		got := seedToPtrInt32(&val)
		if got == nil || *got != 12345 {
			t.Errorf("expected 12345, got %v", got)
		}
	})

	t.Run("dereferenceSeed: nil の場合は 0 を返すのだ", func(t *testing.T) {
		if got := dereferenceSeed(nil); got != 0 {
			t.Errorf("expected 0, got %v", got)
		}
	})

	t.Run("dereferenceSeed: 値がある場合はその値を返すのだ", func(t *testing.T) {
		var val int64 = 999
		if got := dereferenceSeed(&val); got != 999 {
			t.Errorf("expected 999, got %v", got)
		}
	})
}

func TestIsSafeURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"正常なパブリックURL", "https://www.google.com/favicon.ico", false},
		{"不正なスキーム", "gopher://example.com", true},
		{"ループバック", "http://localhost/admin", true},
		{"プライベートIP (クラスA)", "http://10.255.255.254/metadata", true},
		{"名前解決できないドメイン", "http://this.should.not.exist.invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			safe, err := IsSafeURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("isSafeURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !safe {
				t.Error("safe URL was flagged as unsafe")
			}
			if tt.wantErr && safe {
				t.Error("unsafe URL was flagged as safe")
			}
		})
	}
}
