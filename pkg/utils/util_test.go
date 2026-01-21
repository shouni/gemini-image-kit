package utils

import (
	"testing"
)

func TestSeedUtils(t *testing.T) {
	t.Run("dereferenceSeed: nil の場合は 0 を返すのだ", func(t *testing.T) {
		if got := DereferenceSeed(nil); got != 0 {
			t.Errorf("expected 0, got %v", got)
		}
	})

	t.Run("dereferenceSeed: 値がある場合はその値を返すのだ", func(t *testing.T) {
		var val int64 = 999
		if got := DereferenceSeed(&val); got != 999 {
			t.Errorf("expected 999, got %v", got)
		}
	})
}
