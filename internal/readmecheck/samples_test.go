// Package readmecheck compiles the code samples from README.md so the documentation
// cannot drift from the API without CI noticing. Nothing here touches the network:
// the samples only need to type-check.
//
// Interface parameters that the samples do not use are named _ here while the README
// spells them out — a reader learning the interface benefits from the names, and the
// guarantee this file provides is about API shape, not parameter naming.
package readmecheck

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/shouni/go-gemini-client/gemini"

	"github.com/shouni/gemini-image-kit/generator"
	"github.com/shouni/gemini-image-kit/ports"
)

// TestREADMESamplesCompile exists for its compile-time effect: if a README sample stops
// matching the API, this file stops building.
func TestREADMESamplesCompile(t *testing.T) {
	t.Log("README samples type-check against the current API")
}

// --- README「補助実装（最小のプレースホルダ）」そのまま ---

type httpDownloader struct {
	client *http.Client
}

func (d httpDownloader) GetStream(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, errors.New("failed to download image")
	}
	return resp.Body, nil
}

type noStorageReader struct{}

func (noStorageReader) Open(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, errors.New("storage reader is not configured")
}

type memoryCache struct {
	mu   sync.RWMutex
	data map[string]any
}

func newMemoryCache() *memoryCache {
	return &memoryCache{data: make(map[string]any)}
}

func (c *memoryCache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

func (c *memoryCache) Set(key string, value any, _ time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

// 注入する 3 つは README が示すとおりポートを満たすこと。
var (
	_ ports.Downloader    = httpDownloader{}
	_ ports.ContentReader = noStorageReader{}
	_ ports.ImageCacher   = (*memoryCache)(nil)
)

// --- Quick Start 1: Gemini API で単一参照画像から生成する ---

func sampleSingleReference(ctx context.Context) error {
	ai, err := gemini.NewClient(ctx, gemini.Config{APIKey: os.Getenv("GEMINI_API_KEY")})
	if err != nil {
		return err
	}

	core, err := generator.NewGeminiImageCore(generator.GeminiImageCoreConfig{
		AIClient:   ai,
		Reader:     noStorageReader{},
		HTTPClient: httpDownloader{client: http.DefaultClient},
		Cache:      newMemoryCache(),
		CacheTTL:   24 * time.Hour,
		Compress:   true,
	})
	if err != nil {
		return err
	}

	g, err := generator.NewGeminiGenerator(core)
	if err != nil {
		return err
	}

	resp, err := g.Generate(ctx, ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{
			Model:          "gemini-3-pro-image",
			Prompt:         "参照画像の人物を、白背景の商品広告風ポートレートにしてください。",
			NegativePrompt: "low quality, blurry, distorted hands",
			GenerateOptions: gemini.GenerateOptions{
				AspectRatio: "1:1",
				ImageSize:   "1K",
			},
		},
		Images: []ports.ImageURI{{
			ReferenceURL: "https://example.com/reference.png",
		}},
	})
	if err != nil {
		return err
	}
	// ImageResponse の中身の表に載せた全フィールド。
	_, _, _ = resp.MimeType, resp.Model, resp.Prompt
	_, _ = resp.UsedSeed, resp.Usage
	return os.WriteFile("output.png", resp.Data, 0o644)
}

// --- Quick Start 2: 複数の参照画像を統合して生成する ---

func sampleFusion(ctx context.Context, g *generator.GeminiGenerator) error {
	_, err := g.Generate(ctx, ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{
			Model:  "gemini-3-pro-image",
			Prompt: "1枚目のキャラクターを、2枚目の服装と3枚目の背景に自然に合成してください。",
			GenerateOptions: gemini.GenerateOptions{
				AspectRatio: "16:9",
				ImageSize:   "2K",
			},
		},
		Images: []ports.ImageURI{
			{ReferenceURL: "https://example.com/character.png"},
			{ReferenceURL: "https://example.com/outfit.png"},
			{ReferenceURL: "https://example.com/background.png"},
		},
	})
	return err
}

// --- Quick Start 3: Vertex AI で GCS 画像を直接参照する ---

func sampleVertexGCS(ctx context.Context) error {
	ai, err := gemini.NewClient(ctx, gemini.Config{
		ProjectID:  "your-google-cloud-project-id",
		LocationID: "asia-northeast1",
	})
	if err != nil {
		return err
	}
	core, err := generator.NewGeminiImageCore(generator.GeminiImageCoreConfig{
		AIClient:   ai,
		Reader:     noStorageReader{},
		HTTPClient: httpDownloader{client: http.DefaultClient},
		Cache:      newMemoryCache(),
		CacheTTL:   24 * time.Hour,
		Compress:   false,
	})
	if err != nil {
		return err
	}
	g, err := generator.NewGeminiGenerator(core)
	if err != nil {
		return err
	}
	_, err = g.Generate(ctx, ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{
			Model:  "gemini-3-pro-image",
			Prompt: "この商品画像を、SNS 広告向けの高級感ある構図にしてください。",
			GenerateOptions: gemini.GenerateOptions{
				AspectRatio: "4:5",
				ImageSize:   "1K",
			},
		},
		Images: []ports.ImageURI{{
			ReferenceURL: "gs://your-bucket/products/source.png",
		}},
	})
	return err
}

// --- Quick Start 4: 既存画像を編集する ---

func sampleEdit(ctx context.Context, g *generator.GeminiGenerator) error {
	resp, err := g.Generate(ctx, ports.ImageRequest{
		GenerationOptions: ports.GenerationOptions{
			Model:  "gemini-3.1-flash-image",
			Prompt: "対象領域のバッグを黒いレザーバッグに差し替えてください。他の部分は変更しないでください。",
		},
		Images: []ports.ImageURI{{
			ReferenceURL: "gs://your-bucket/edit/source.png",
		}},
	})
	if err != nil {
		return err
	}
	return os.WriteFile("edited.png", resp.Data, 0o644)
}

// --- Public API 節: オプションと一括生成 ---

func sampleOptionsAndBatch(ctx context.Context, core *generator.GeminiImageCore) error {
	g, err := generator.NewGeminiGenerator(core,
		generator.WithRateLimit(30*time.Second, 1),
		generator.WithMaxConcurrency(2),
		generator.WithRequestTimeout(5*time.Minute),
		generator.WithoutAutoSeed(),
	)
	if err != nil {
		return err
	}
	// Public API 節に載せた 2 つのシグネチャ。
	var _ ports.ImageGenerator = g
	var _ ports.BatchImageGenerator = g
	_, err = g.GenerateBatch(ctx, []ports.ImageRequest{
		{GenerationOptions: ports.GenerationOptions{Model: "m", Prompt: "p"}},
	})
	return err
}

// sampleConfigFields pins every row of the README's config table.
func sampleConfigFields() {
	_ = generator.GeminiImageCoreConfig{
		AIClient:           nil,
		Reader:             noStorageReader{},
		HTTPClient:         httpDownloader{},
		Cache:              newMemoryCache(),
		CacheTTL:           24 * time.Hour,
		Compress:           true,
		CompressionQuality: generator.DefaultCompressionQuality,
		UploadTimeout:      generator.DefaultUploadTimeout,
		InlineReferences:   false,
		FetchTimeout:       generator.DefaultFetchTimeout,
		MaxReferenceBytes:  generator.DefaultMaxReferenceBytes,
		Logger:             nil,
	}
}

// sampleSentinels pins every sentinel named in the error-handling section.
func sampleSentinels() []error {
	return []error{
		generator.ErrModelRequired, generator.ErrEmptyPrompt,
		generator.ErrUnsupportedFileFormat, generator.ErrReferenceTooLarge,
		generator.ErrNoImageData, generator.ErrAIClientRequired,
		generator.ErrReaderRequired, generator.ErrHTTPClientRequired,
		generator.ErrCacheRequired, generator.ErrExecutorRequired,
	}
}

// 参照されない関数を unused 検出から守るための参照点。
var _ = []any{
	sampleSingleReference, sampleFusion, sampleVertexGCS, sampleEdit,
	sampleOptionsAndBatch, sampleConfigFields, sampleSentinels,
}
