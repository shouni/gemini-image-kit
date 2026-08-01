package generator

import (
	"testing"

	"github.com/shouni/go-gemini-client/gemini"
)

// parseToResponse のテスト
func TestGeminiImageCore_parseToResponse(t *testing.T) {
	core := &GeminiImageCore{}
	seed := int64(999)

	t.Run("正常系: 画像が返る", func(t *testing.T) {
		resp := &gemini.Response{
			Attachments: []gemini.Attachment{{MIMEType: "image/png", Data: []byte("png-data")}},
		}

		out, err := core.parseToResponse(resp, seed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.MimeType != "image/png" || out.UsedSeed != seed {
			t.Errorf("parsed data mismatch: %+v", out)
		}
	})

	// NOTE: FinishReason の検証（安全ブロック等）は go-gemini-client 側が行うため、
	// parseToResponse では画像データの有無のみを検証します。
	t.Run("異常系: 添付が空（ブロック等で画像が返らなかった場合）", func(t *testing.T) {
		resp := &gemini.Response{}
		_, err := core.parseToResponse(resp, seed)
		if err == nil {
			t.Error("expected error when no image data is present")
		}
	})

	t.Run("異常系: 画像データなし（テキストのみ）", func(t *testing.T) {
		resp := &gemini.Response{Text: "just text"}
		_, err := core.parseToResponse(resp, seed)
		if err == nil {
			t.Error("expected error for text-only response")
		}
	})

	t.Run("異常系: 空のレスポンス", func(t *testing.T) {
		_, err := core.parseToResponse(nil, seed)
		if err == nil {
			t.Error("expected error for nil response")
		}
	})
}
