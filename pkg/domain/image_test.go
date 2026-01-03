package domain

import (
	"testing"
)

func TestImageGenerationRequest_Seed(t *testing.T) {
	// 指摘対応：テストケース名を英語にし、客観性を確保
	t.Run("should handle nil seed as random", func(t *testing.T) {
		req := ImageGenerationRequest{
			Prompt: "走るずんだもん",
			Seed:   nil, // *int64 なので nil 代入可能
		}

		if req.Seed != nil {
			t.Error("Seed should be nil")
		}
	})

	t.Run("should correctly store a specified seed value", func(t *testing.T) {
		var val int64 = 9223372036854775807 // MaxInt64 で精度を確認
		req := ImageGenerationRequest{
			Prompt: "笑うずんだもん",
			Seed:   &val, // アドレスを渡す
		}

		if req.Seed == nil {
			t.Fatalf("Seed should not be nil")
		}

		// [指摘対応] ポインタのアドレスではなく、中身の値を比較・出力
		if *req.Seed != val {
			t.Errorf("Seed value is incorrect. want: %d, got: %d", val, *req.Seed)
		}
	})
}

func TestImageResponse_TypeConsistency(t *testing.T) {
	t.Run("should maintain int64 precision for the used seed", func(t *testing.T) {
		var largeSeed int64 = 9223372036854775807
		resp := ImageResponse{
			Data:     []byte{0xFF, 0xD8},
			MimeType: "image/jpeg",
			UsedSeed: largeSeed,
		}

		if resp.UsedSeed != largeSeed {
			t.Errorf("Large seed value precision was lost. want: %d, got: %d", largeSeed, resp.UsedSeed)
		}
	})
}
