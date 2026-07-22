package generator

import "errors"

var (
	// ErrFileNotInCache は、File API 上のファイル名がキャッシュから引けず削除できない場合に返されます。
	ErrFileNotInCache = errors.New("cannot determine file name for deletion, file not found in cache")
	// ErrModelRequired は、生成リクエストにモデル名が指定されていない場合に返されます。
	ErrModelRequired = errors.New("model is required")
	// ErrEmptyPrompt は、プロンプト（ネガティブプロンプト含む）が空の場合に返されます。
	ErrEmptyPrompt = errors.New("prompt cannot be empty")
	// ErrUnsupportedFileFormat は、取得したデータが画像として扱えない場合に返されます。
	ErrUnsupportedFileFormat = errors.New("unsupported file format")
	// ErrEmptyResponse は、Gemini からのレスポンスが空または不正な場合に返されます。
	ErrEmptyResponse = errors.New("invalid or empty response from Gemini")
	// ErrNoImageData は、レスポンスに画像データが含まれていない場合に返されます。
	ErrNoImageData = errors.New("no image data found in response")
	// ErrAIClientRequired は、NewGeminiImageCore に aiClient が渡されなかった場合に返されます。
	ErrAIClientRequired = errors.New("aiClient is required")
	// ErrReaderRequired は、NewGeminiImageCore に reader が渡されなかった場合に返されます。
	ErrReaderRequired = errors.New("reader is required")
	// ErrHTTPClientRequired は、NewGeminiImageCore に httpClient が渡されなかった場合に返されます。
	ErrHTTPClientRequired = errors.New("httpClient is required")
	// ErrExecutorRequired は、NewGeminiGenerator に core (ImageExecutor) が渡されなかった場合に返されます。
	ErrExecutorRequired = errors.New("core (ImageExecutor) is required")
)
