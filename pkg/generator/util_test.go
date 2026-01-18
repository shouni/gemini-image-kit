package generator

import "testing"

func TestSeedUtils(t *testing.T) {
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
		{"GCSスキーム (gs://)", "gs://my-bucket/path/to/image.png", false},

		{"不正なスキーム", "gopher://example.com", true},
		{"ループバック", "http://localhost/admin", true},
		{"プライベートIP (クラスA)", "http://10.255.255.254/metadata", true},
		{"名前解決できないドメイン", "http://this.should.not.exist.invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			safe, err := IsSafeURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsSafeURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !safe {
				t.Errorf("%s: safe URL was flagged as unsafe", tt.url)
			}
			if tt.wantErr && safe {
				t.Errorf("%s: unsafe URL was flagged as safe", tt.url)
			}
		})
	}
}
