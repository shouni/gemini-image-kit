package generator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shouni/go-gemini-client/pkg/gemini"
	"google.golang.org/genai"
)

func (c *GeminiImageCore) fetchImageData(ctx context.Context, rawURL string) ([]byte, error) {
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

func (c *GeminiImageCore) dereferenceSeed(seed *int64) int64 {
	if seed == nil {
		return 0
	}
	return *seed
}
