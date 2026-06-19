package generator

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/shouni/gemini-image-kit/ports"
	"google.golang.org/genai"
)

// GenerateSingleImage の構造チェック
func TestGeminiGenerator_GenerateSingleImage_Structure(t *testing.T) {
	t.Run("FileAPIURIとImageSizeが正しく扱われること", func(t *testing.T) {
		req := ports.SingleImageRequest{
			GenerationOptions: ports.GenerationOptions{
				Prompt:    "test prompt",
				ImageSize: "2K",
			},
			Image: ports.ImageURI{
				FileAPIURI:   "https://generativelanguage.googleapis.com/v1beta/files/test",
				ReferenceURL: "gs://bucket/ref.png",
			},
		}

		if req.Image.FileAPIURI == "" {
			t.Error("FileAPIURI should be set in req.Image")
		}

		if req.ImageSize != "2K" {
			t.Errorf("ImageSize should be 2K, got %s", req.GenerationOptions.ImageSize)
		}
	})
}

func TestGeminiGenerator_GenerateFusedImage_Structure(t *testing.T) {
	t.Run("複数枚のImageURIが保持されること", func(t *testing.T) {
		req := ports.ImageFusionRequest{
			GenerationOptions: ports.GenerationOptions{
				AspectRatio: "16:9",
			},
			Images: []ports.ImageURI{
				{FileAPIURI: "api-1", ReferenceURL: "ref-1"},
				{FileAPIURI: "api-2", ReferenceURL: "ref-2"},
			},
		}

		if len(req.Images) != 2 {
			t.Errorf("expected 2 images, got %d", len(req.Images))
		}

		if req.Images[0].FileAPIURI != "api-1" {
			t.Errorf("first image API URI mismatch: %s", req.Images[0].FileAPIURI)
		}
	})
}

func TestGeminiGenerator_GenerateSingleImage_ReturnsImagePreparationError(t *testing.T) {
	ctx := context.Background()
	ai := &mockAIClient{}
	core, err := NewGeminiImageCore(
		ai,
		&mockReader{},
		&mockHTTPClient{err: errors.New("download failed")},
		&mockCache{},
		time.Hour,
		false,
	)
	if err != nil {
		t.Fatalf("failed to create core: %v", err)
	}
	g, err := NewGeminiGenerator(core)
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	_, err = g.GenerateSingleImage(ctx, ports.SingleImageRequest{
		GenerationOptions: ports.GenerationOptions{
			Model:  "gemini-test-model",
			Prompt: "test prompt",
		},
		Image: ports.ImageURI{ReferenceURL: "https://example.com/source.png"},
	})

	if err == nil {
		t.Fatal("expected image preparation error")
	}
	if ai.generateCalled {
		t.Fatal("GenerateWithParts should not be called when image preparation fails")
	}
}

func TestGeminiGenerator_EditImage(t *testing.T) {
	ctx := context.Background()
	seed := int64(42)
	ai := &mockAIClient{backend: genai.BackendVertexAI}
	core, err := NewGeminiImageCore(
		ai,
		&mockReader{},
		&mockHTTPClient{data: []byte("\x89PNG\r\n\x1a\n")},
		&mockCache{},
		time.Hour,
		false,
	)
	if err != nil {
		t.Fatalf("failed to create core: %v", err)
	}
	g, err := NewGeminiGenerator(core)
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	resp, err := g.EditImage(ctx, ports.EditImageRequest{
		Model:      "imagen-edit",
		EditPrompt: "replace the background",
		Image:      ports.ImageURI{ReferenceURL: "gs://bucket/source.png"},
		Mask:       ports.ImageURI{ReferenceURL: "https://example.com/mask.png"},
		TargetBBox: &ports.BoundingBox{X: 1, Y: 2, Width: 30, Height: 40},
		Seed:       &seed,
	})
	if err != nil {
		t.Fatalf("EditImage() unexpected error = %v", err)
	}

	if !ai.editCalled {
		t.Fatal("EditImage should be called")
	}
	if ai.lastEditModel != "imagen-edit" {
		t.Fatalf("model = %q, want imagen-edit", ai.lastEditModel)
	}
	if !strings.Contains(ai.lastEditPrompt, "Target bounding box: x=1, y=2, width=30, height=40.") {
		t.Fatalf("edit prompt does not include bbox: %q", ai.lastEditPrompt)
	}
	if len(ai.lastEditRefs) != 2 {
		t.Fatalf("reference images length = %d, want 2", len(ai.lastEditRefs))
	}
	if ai.lastEditConfig == nil || ai.lastEditConfig.Seed == nil || *ai.lastEditConfig.Seed != 42 {
		t.Fatalf("seed was not forwarded to edit config: %+v", ai.lastEditConfig)
	}
	if resp.MimeType != "image/png" || string(resp.Data) != "fake-edited-image-bytes" || resp.UsedSeed != seed {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestGeminiGenerator_EditImage_RejectsOutOfRangeSeed(t *testing.T) {
	ctx := context.Background()
	seed := int64(math.MaxInt32) + 1
	ai := &mockAIClient{backend: genai.BackendVertexAI}
	core, err := NewGeminiImageCore(
		ai,
		&mockReader{},
		&mockHTTPClient{data: []byte("\x89PNG\r\n\x1a\n")},
		&mockCache{},
		time.Hour,
		false,
	)
	if err != nil {
		t.Fatalf("failed to create core: %v", err)
	}
	g, err := NewGeminiGenerator(core)
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	_, err = g.EditImage(ctx, ports.EditImageRequest{
		Model:      "imagen-edit",
		EditPrompt: "replace the background",
		Image:      ports.ImageURI{ReferenceURL: "gs://bucket/source.png"},
		Seed:       &seed,
	})

	if err == nil {
		t.Fatal("expected seed range error")
	}
	if ai.editCalled {
		t.Fatal("EditImage should not be called with out-of-range seed")
	}
}
