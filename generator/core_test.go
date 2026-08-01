package generator

import (
	"testing"
	"time"

	"github.com/shouni/go-gemini-client/gemini"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shouni/gemini-image-kit/ports"
)

// TestNewGeminiImageCore_RequiredDependencies は、必須依存の nil チェックを検証します。
//
// cache は「あれば速くなる」任意の依存に見えますが、DeleteFile が File API 上の
// ファイル名をここから引くため、nil だと削除が一切できずサーバー側にファイルが
// 残り続けます。したがって必須です。
func TestNewGeminiImageCore_RequiredDependencies(t *testing.T) {
	ai := &mockAIClient{}
	reader := &mockReader{}
	httpClient := &mockHTTPClient{}
	cache := &mockCache{}

	tests := []struct {
		name    string
		ai      gemini.MultimodalModel
		reader  ports.ContentReader
		http    ports.Downloader
		cache   ports.ImageCacher
		wantErr error
	}{
		{"aiClient なし", nil, reader, httpClient, cache, ErrAIClientRequired},
		{"reader なし", ai, nil, httpClient, cache, ErrReaderRequired},
		{"httpClient なし", ai, reader, nil, cache, ErrHTTPClientRequired},
		{"cache なし", ai, reader, httpClient, nil, ErrCacheRequired},
		{"すべて揃っている", ai, reader, httpClient, cache, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, err := NewGeminiImageCore(GeminiImageCoreConfig{
				AIClient: tt.ai, Reader: tt.reader, HTTPClient: tt.http,
				Cache: tt.cache, CacheTTL: time.Hour, Compress: false,
			})

			if tt.wantErr == nil {
				require.NoError(t, err)
				assert.NotNil(t, core)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, core)
		})
	}
}
