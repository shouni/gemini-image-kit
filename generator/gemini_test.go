package generator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"google.golang.org/genai"

	"github.com/shouni/gemini-image-kit/ports"
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
			t.Errorf("ImageSize should be 2K, got %s", req.ImageSize)
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

func TestToOptions_PersonGeneration(t *testing.T) {
	newGenerator := func(t *testing.T, backend genai.Backend) *GeminiGenerator {
		t.Helper()
		core, err := NewGeminiImageCore(
			&mockAIClient{backend: backend},
			&mockReader{},
			&mockHTTPClient{},
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
		return g
	}

	t.Run("Vertex AI: 未指定なら AllowAll", func(t *testing.T) {
		g := newGenerator(t, genai.BackendVertexAI)
		opts := g.toOptions(ports.GenerationOptions{})
		if opts.PersonGeneration != gemini.PersonGenerationAllowAll {
			t.Errorf("PersonGeneration = %q, want %q", opts.PersonGeneration, gemini.PersonGenerationAllowAll)
		}
	})

	t.Run("Vertex AI: 指定された値を尊重する", func(t *testing.T) {
		g := newGenerator(t, genai.BackendVertexAI)
		opts := g.toOptions(ports.GenerationOptions{
			PersonGeneration: gemini.PersonGenerationDontAllow,
		})
		if opts.PersonGeneration != gemini.PersonGenerationDontAllow {
			t.Errorf("PersonGeneration = %q, want %q", opts.PersonGeneration, gemini.PersonGenerationDontAllow)
		}
	})

	t.Run("Gemini API: 指定されていても常に未設定", func(t *testing.T) {
		g := newGenerator(t, genai.BackendGeminiAPI)
		opts := g.toOptions(ports.GenerationOptions{
			PersonGeneration: gemini.PersonGenerationAllowAll,
		})
		if opts.PersonGeneration != gemini.PersonGenerationUnspecified {
			t.Errorf("PersonGeneration = %q, want unspecified", opts.PersonGeneration)
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
