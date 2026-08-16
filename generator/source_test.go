package generator

import (
	"testing"
)

// TestUploadDisplayName は、署名付き URL のクエリ文字列が表示名に混ざらないことを
// 確認します。filepath.Base をそのまま掛けると "ref.png?X-Goog-Signature=..." になります。
func TestUploadDisplayName(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{name: "GCS URI", uri: "gs://bucket/dir/ref.png", want: "ref.png"},
		{name: "署名付き URL", uri: "https://example.com/a/ref.png?X-Goog-Signature=abc&x=1", want: "ref.png"},
		{name: "フラグメント付き", uri: "https://example.com/ref.jpg#frag", want: "ref.jpg"},
		{name: "スキーム無し", uri: "dir/ref.webp", want: "ref.webp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uploadDisplayName(tt.uri); got != tt.want {
				t.Errorf("uploadDisplayName(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}
