package generator

import (
	"testing"
)

func TestBuildFinalPrompt(t *testing.T) {
	// 実装側の negativePromptSeparator と一致させる
	const sep = negativePromptSeparator

	tests := []struct {
		name     string
		prompt   string
		negative string
		expected string
	}{
		{
			name:     "プロンプトのみ",
			prompt:   "a cute cat",
			negative: "",
			expected: "a cute cat",
		},
		{
			name:     "両方あり",
			prompt:   "a cute cat",
			negative: "blurry, low quality",
			expected: "a cute cat" + sep + "blurry, low quality",
		},
		{
			name:     "空の入力",
			prompt:   "  ",
			negative: "",
			expected: "",
		},
		{
			name:     "トリミング確認",
			prompt:   "  spaced prompt  ",
			negative: "  spaced negative  ",
			expected: "spaced prompt" + sep + "spaced negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFinalPrompt(tt.prompt, tt.negative)
			if got != tt.expected {
				t.Errorf("buildFinalPrompt() = %q; want %q", got, tt.expected)
			}
		})
	}
}
