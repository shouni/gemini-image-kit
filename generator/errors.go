package generator

import "errors"

var (
	// ErrModelRequired は、生成リクエストにモデル名が指定されていない場合に返されます。
	ErrModelRequired = errors.New("imagekit: model is required")
	// ErrEmptyPrompt は、プロンプト（ネガティブプロンプト含む）が空の場合に返されます。
	ErrEmptyPrompt = errors.New("imagekit: prompt cannot be empty")
	// ErrUnsupportedFileFormat は、取得したデータが画像として扱えない場合に返されます。
	ErrUnsupportedFileFormat = errors.New("imagekit: unsupported file format")
	// ErrReferenceTooLarge は、参照画像が MaxReferenceBytes を超えている場合に返されます。
	ErrReferenceTooLarge = errors.New("imagekit: reference image too large")
	// ErrNoImageData は、レスポンスに画像データが含まれていない場合に返されます。
	ErrNoImageData = errors.New("imagekit: no image data found in response")
	// ErrAIClientRequired は、NewGeminiImageCore に AIClient が渡されなかった場合に返されます。
	ErrAIClientRequired = errors.New("imagekit: AIClient is required")
	// ErrReaderRequired は、NewGeminiImageCore に Reader が渡されなかった場合に返されます。
	ErrReaderRequired = errors.New("imagekit: Reader is required")
	// ErrHTTPClientRequired は、NewGeminiImageCore に HTTPClient が渡されなかった場合に返されます。
	ErrHTTPClientRequired = errors.New("imagekit: HTTPClient is required")
	// ErrCacheRequired は、NewGeminiImageCore に Cache が渡されなかった場合に返されます。
	// アップロード済み参照の使い回しがこのキットの主要なコスト最適化であり、
	// キャッシュ無しでは同じ参照画像を毎回アップロードし直すことになります。
	ErrCacheRequired = errors.New("imagekit: Cache is required")
	// ErrExecutorRequired は、NewGeminiGenerator に core が渡されなかった場合に返されます。
	ErrExecutorRequired = errors.New("imagekit: core is required")
)
