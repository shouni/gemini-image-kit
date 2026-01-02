package domain

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCharacter_JSON(t *testing.T) {
	t.Run("Character構造体が正しくJSON変換できるのだ", func(t *testing.T) {
		char := Character{
			ID:           "zundamon-001",
			Name:         "ずんだもん",
			VisualCues:   []string{"green hair", "zunda mochi ears"},
			ReferenceURL: "http://example.com/zundamon.png",
			Seed:         123456789012345, // int64の範囲
		}

		data, err := json.Marshal(char)
		if err != nil {
			t.Fatalf("Marshal失敗なのだ: %v", err)
		}

		var decoded Character
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal失敗なのだ: %v", err)
		}

		if !reflect.DeepEqual(char, decoded) {
			t.Errorf("変換前後でデータが一致しないのだ。期待: %+v, 実際: %+v", char, decoded)
		}
	})
}

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

func TestMangaResponse_JSON(t *testing.T) {
	t.Run("AIからのレスポンス形式をシミュレートするのだ", func(t *testing.T) {
		inputJSON := `{
			"title": "ずんだもんの冒険",
			"pages": [
				{
					"page": 1,
					"visual_anchor": "森の中",
					"dialogue": "のだ！",
					"speaker_id": "zundamon"
				}
			]
		}`

		var resp MangaResponse
		if err := json.Unmarshal([]byte(inputJSON), &resp); err != nil {
			t.Fatalf("パース失敗なのだ: %v", err)
		}

		if resp.Title != "ずんだもんの冒険" {
			t.Errorf("タイトルが違うのだ: %s", resp.Title)
		}
		if len(resp.Pages) != 1 || resp.Pages[0].Dialogue != "のだ！" {
			t.Error("ページ内容が正しくパースされていないのだ")
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
