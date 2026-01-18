package generator

import (
	"testing"

	"github.com/shouni/gemini-image-kit/pkg/domain"
)

// buildFinalPrompt の単体テスト
func TestBuildFinalPrompt(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		negative string
		want     string
	}{
		{
			name:     "プロンプトのみ",
			prompt:   "a cute robot",
			negative: "",
			want:     "a cute robot",
		},
		{
			name:     "ネガティブプロンプトあり",
			prompt:   "a cute robot",
			negative: "blurry, low quality",
			want:     "a cute robot\n\n[Negative Prompt]\nblurry, low quality",
		},
		{
			name:     "空文字とスペースのトリミング",
			prompt:   "  starry night  ",
			negative: "  dark  ",
			want:     "starry night\n\n[Negative Prompt]\ndark",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFinalPrompt(tt.prompt, tt.negative)
			if got != tt.want {
				t.Errorf("buildFinalPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

// GenerateMangaPanel の構造チェック（モックを利用する想定）
func TestGeminiGenerator_GenerateMangaPanel_Structure(t *testing.T) {
	// 実際には core の依存関係（aiClient, httpClient等）をモック化した core を作成します
	// ここでは、インターフェースではなく具象型に依存しているため、
	// 依存先の Mock を NewGeminiImageCore に注入してテストします。

	t.Run("FileAPIURIが優先されること", func(t *testing.T) {
		// ※ ここではロジックの「流れ」を確認する擬似的なコードです
		// 実際には mockClient を作成して、期待される parts が渡っているか検証します

		req := domain.ImageGenerationRequest{
			Prompt:     "test prompt",
			FileAPIURI: "https://generativelanguage.googleapis.com/v1beta/files/test",
		}

		// 本来は core.executeRequest をフックして、
		// parts[1].FileData.FileURI == req.FileAPIURI であることを確認します。
		_ = req
	})
}
