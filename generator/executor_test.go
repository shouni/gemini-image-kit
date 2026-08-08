package generator

import (
	"testing"

	"github.com/shouni/go-gemini-client/gemini"
)

// parseToResponse のテスト
func TestParseToResponse(t *testing.T) {
	seed := int64(999)

	t.Run("正常系: 画像が返る", func(t *testing.T) {
		resp := &gemini.Response{
			Attachments: []gemini.Attachment{{MIMEType: "image/png", Data: []byte("png-data")}},
		}

		out, err := parseToResponse(resp, "gemini-test-model", "final prompt", seed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.MimeType != "image/png" || out.UsedSeed != seed {
			t.Errorf("parsed data mismatch: %+v", out)
		}
		if out.Model != "gemini-test-model" || out.Prompt != "final prompt" {
			t.Errorf("model/prompt metadata missing: %+v", out)
		}
	})

	// NOTE: FinishReason の検証（安全ブロック等）は go-gemini-client 側が行うため、
	// parseToResponse では画像データの有無のみを検証します。
	t.Run("異常系: 添付が空（ブロック等で画像が返らなかった場合）", func(t *testing.T) {
		resp := &gemini.Response{}
		_, err := parseToResponse(resp, "m", "p", seed)
		if err == nil {
			t.Error("expected error when no image data is present")
		}
	})

	t.Run("異常系: 画像データなし（テキストのみ）", func(t *testing.T) {
		resp := &gemini.Response{Text: "just text"}
		_, err := parseToResponse(resp, "m", "p", seed)
		if err == nil {
			t.Error("expected error for text-only response")
		}
	})

	t.Run("異常系: 空のレスポンス", func(t *testing.T) {
		_, err := parseToResponse(nil, "m", "p", seed)
		if err == nil {
			t.Error("expected error for nil response")
		}
	})
}
