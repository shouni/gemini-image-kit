package generator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shouni/gemini-image-kit/pkg/domain"
	"github.com/shouni/gemini-image-kit/pkg/imgutil"
	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
)

func (c *GeminiImageCore) executeRequest(ctx context.Context, model string, parts []*genai.Part, opts gemini.GenerateOptions) (*domain.ImageResponse, error) {
	gOpts := gemini.GenerateOptions{
		AspectRatio:  opts.AspectRatio,
		SystemPrompt: opts.SystemPrompt,
		Seed:         opts.Seed,
	}

	resp, err := c.aiClient.GenerateWithParts(ctx, model, parts, gOpts)
	if err != nil {
		return nil, err
	}

	out, err := c.parseToResponse(resp, dereferenceSeed(opts.Seed))
	if err != nil {
		return nil, err
	}

	return &domain.ImageResponse{
		Data:     out.Data,
		MimeType: out.MimeType,
		UsedSeed: out.UsedSeed,
	}, nil
}

func (c *GeminiImageCore) prepareImagePart(ctx context.Context, rawURL string) *genai.Part {
	// File API キャッシュチェック
	if c.cache != nil {
		if val, ok := c.cache.Get(cacheKeyFileAPIURI + rawURL); ok {
			if uri, ok := val.(string); ok {
				return &genai.Part{FileData: &genai.FileData{FileURI: uri}}
			}
		}
	}

	// 取得と圧縮
	data, err := c.fetchImageData(ctx, rawURL)
	if err != nil {
		return nil
	}
	finalData := data
	if UseImageCompression {
		if compressed, err := imgutil.CompressToJPEG(data, ImageCompressionQuality); err == nil {
			finalData = compressed
		}
	}

	return c.toPart(finalData)
}

func (c *GeminiImageCore) fetchImageData(ctx context.Context, rawURL string) ([]byte, error) {
	if safe, err := IsSafeURL(rawURL); err != nil || !safe {
		return nil, fmt.Errorf("安全ではないURLが指定されました: %w", err)
	}

	if strings.HasPrefix(rawURL, "gs://") {
		rc, err := c.reader.Open(ctx, rawURL)
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return c.httpClient.FetchBytes(ctx, rawURL)
}

func (c *GeminiImageCore) toPart(data []byte) *genai.Part {
	mimeType := http.DetectContentType(data)
	if !strings.HasPrefix(mimeType, "image/") {
		return nil
	}
	return &genai.Part{InlineData: &genai.Blob{MIMEType: mimeType, Data: data}}
}

func (c *GeminiImageCore) parseToResponse(resp *gemini.Response, seed int64) (*ImageOutput, error) {
	if resp == nil || resp.RawResponse == nil || len(resp.RawResponse.Candidates) == 0 {
		return nil, fmt.Errorf("invalid response")
	}
	candidate := resp.RawResponse.Candidates[0]
	for _, part := range candidate.Content.Parts {
		if part.InlineData != nil {
			return &ImageOutput{Data: part.InlineData.Data, MimeType: part.InlineData.MIMEType, UsedSeed: seed}, nil
		}
	}
	return nil, fmt.Errorf("no image data")
}
