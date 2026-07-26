package generator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/ports"
)

func TestGeminiGenerator_GenerateSingleImage(t *testing.T) {
	ctx := context.Background()
	ai := &mockAIClient{}
	core, err := NewGeminiImageCore(GeminiImageCoreConfig{
		AIClient: ai, Reader: &mockReader{}, HTTPClient: &mockHTTPClient{},
		Cache: &mockCache{}, CacheTTL: time.Hour, Compress: false,
	})
	if err != nil {
		t.Fatalf("failed to create core: %v", err)
	}
	g, err := NewGeminiGenerator(core)
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	resp, err := g.GenerateSingleImage(ctx, ports.SingleImageRequest{
		GenerationOptions: ports.GenerationOptions{
			Model:     "gemini-test-model",
			Prompt:    "test prompt",
			ImageSize: "2K",
		},
		Image: ports.ImageURI{
			FileAPIURI:   "https://generativelanguage.googleapis.com/v1beta/files/test",
			ReferenceURL: "gs://bucket/ref.png",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ai.generateCalled {
		t.Fatal("expected GenerateWithParts to be called")
	}
	if resp == nil || resp.MimeType != "image/png" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestGeminiGenerator_GenerateFusedImage(t *testing.T) {
	ctx := context.Background()
	ai := &mockAIClient{}
	core, err := NewGeminiImageCore(GeminiImageCoreConfig{
		AIClient: ai, Reader: &mockReader{}, HTTPClient: &mockHTTPClient{},
		Cache: &mockCache{}, CacheTTL: time.Hour, Compress: false,
	})
	if err != nil {
		t.Fatalf("failed to create core: %v", err)
	}
	g, err := NewGeminiGenerator(core)
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	resp, err := g.GenerateFusedImage(ctx, ports.ImageFusionRequest{
		GenerationOptions: ports.GenerationOptions{
			Model:       "gemini-test-model",
			Prompt:      "fuse these",
			AspectRatio: "16:9",
		},
		Images: []ports.ImageURI{
			{FileAPIURI: "https://generativelanguage.googleapis.com/v1beta/files/one", ReferenceURL: "ref-1"},
			{FileAPIURI: "https://generativelanguage.googleapis.com/v1beta/files/two", ReferenceURL: "ref-2"},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ai.generateCalled {
		t.Fatal("expected GenerateWithParts to be called")
	}
	if resp == nil || resp.MimeType != "image/png" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestToOptions_PersonGeneration(t *testing.T) {
	newGenerator := func(t *testing.T, vertexAI bool) *GeminiGenerator {
		t.Helper()
		core, err := NewGeminiImageCore(GeminiImageCoreConfig{
			AIClient: &mockAIClient{vertexAI: vertexAI}, Reader: &mockReader{}, HTTPClient: &mockHTTPClient{},
			Cache: &mockCache{}, CacheTTL: time.Hour, Compress: false,
		})
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
		g := newGenerator(t, true)
		opts := g.toOptions(ports.GenerationOptions{})
		if opts.PersonGeneration != gemini.PersonGenerationAllowAll {
			t.Errorf("PersonGeneration = %q, want %q", opts.PersonGeneration, gemini.PersonGenerationAllowAll)
		}
	})

	t.Run("Vertex AI: 指定された値を尊重する", func(t *testing.T) {
		g := newGenerator(t, true)
		opts := g.toOptions(ports.GenerationOptions{
			PersonGeneration: gemini.PersonGenerationDontAllow,
		})
		if opts.PersonGeneration != gemini.PersonGenerationDontAllow {
			t.Errorf("PersonGeneration = %q, want %q", opts.PersonGeneration, gemini.PersonGenerationDontAllow)
		}
	})

	t.Run("Gemini API: 指定されていても常に未設定", func(t *testing.T) {
		g := newGenerator(t, false)
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
	core, err := NewGeminiImageCore(GeminiImageCoreConfig{
		AIClient: ai, Reader: &mockReader{}, HTTPClient: &mockHTTPClient{err: errors.New("download failed")},
		Cache: &mockCache{}, CacheTTL: time.Hour, Compress: false,
	})
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
