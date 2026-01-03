package domain

import (
	"testing"
)

func TestImageGenerationRequest_Seed(t *testing.T) {
	t.Run("Seedがnilの場合はランダムとして扱えるのだ", func(t *testing.T) {
		req := ImageGenerationRequest{
			Prompt: "走るずんだもん",
			Seed:   nil,
		}

		if req.Seed != nil {
			t.Error("Seedはnilであるべきなのだ")
		}
	})

	t.Run("Seedに値を指定して固定できるのだ", func(t *testing.T) {
		var val int32 = 42
		req := ImageGenerationRequest{
			Prompt: "笑うずんだもん",
			Seed:   &val,
		}

		if req.Seed == nil || *req.Seed != 42 {
			t.Errorf("Seedが正しく保持されていないのだ。値: %v", req.Seed)
		}
	})
}

func TestImageResponse_TypeConsistency(t *testing.T) {
	t.Run("生成結果のSeedがint64で保持されることを確認するのだ", func(t *testing.T) {
		// UsedSeed は SDK の int32 範囲を超えた値も保持できる必要があるのだ
		var largeSeed int64 = 9223372036854775807 // MaxInt64
		resp := ImageResponse{
			Data:     []byte{0xFF, 0xD8}, // JPEG header dummy
			MimeType: "image/jpeg",
			UsedSeed: largeSeed,
		}

		if resp.UsedSeed != largeSeed {
			t.Errorf("大きなシード値が維持されていないのだ: %d", resp.UsedSeed)
		}
	})
}
