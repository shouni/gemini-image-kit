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
	// ErrUnresolvedReference は、どの resolver も参照を扱えなかった場合に返されます。
	//
	// 経路の設定漏れを示します（例: gs:// 以外も来る構成で GCSResolver しか置いていない）。
	// 受け皿として FetchResolver を最後段に置くと解消します。
	ErrUnresolvedReference = errors.New("imagekit: no resolver could handle the reference")
	// ErrAIClientRequired は、New に AI クライアントが渡されなかった場合に返されます。
	ErrAIClientRequired = errors.New("imagekit: AI client is required")
	// ErrResolverRequired は、New に ReferenceResolver が渡されなかった場合に返されます。
	//
	// 既定値を用意しないのは、参照の送り方（gs:// 直参照 / File API / インライン）が
	// バックエンドと運用で変わり、黙って選ぶと利用側が気付けないためです。
	ErrResolverRequired = errors.New("imagekit: reference resolver is required")
	// ErrVertexAIRequired は、Vertex AI 専用の resolver（GCSResolver）に Gemini API
	// バックエンドのクライアントが組み合わされた場合に New が返します。生成時に
	// 不可解な失敗をするより、構築時に落とします。
	//
	// 判定できるのは gemini.BackendInspector を満たすクライアントだけで、申告が
	// 無い場合は素通しします（詳細は New を参照）。
	ErrVertexAIRequired = errors.New("imagekit: resolver requires the Vertex AI backend")
	// ErrGeminiAPIRequired は、Gemini API 専用の resolver（FileAPIResolver）に
	// Vertex AI のクライアントが組み合わされた場合に New が返します。
	//
	// Vertex AI に File API は無いためアップロードは必ず失敗し、経路は毎回インラインへ
	// 落ちます。生成自体は成功してしまうぶん気付きにくく、gs:// の参照を無駄に 2 回
	// ダウンロードし続けるので、構築時に落とします。
	ErrGeminiAPIRequired = errors.New("imagekit: resolver requires the Gemini API backend")
	// ErrReaderRequired は、resolver に Reader が渡されなかった場合に返されます。
	ErrReaderRequired = errors.New("imagekit: Reader is required")
	// ErrHTTPClientRequired は、resolver に Downloader が渡されなかった場合に返されます。
	ErrHTTPClientRequired = errors.New("imagekit: Downloader is required")
	// ErrFileManagerRequired は、NewFileAPIResolver に Files が渡されなかった場合に返されます。
	ErrFileManagerRequired = errors.New("imagekit: FileManager is required")
	// ErrCacheRequired は、NewFileAPIResolver に Cache が渡されなかった場合に返されます
	// （必須である理由は FileAPIResolverConfig.Cache を参照）。
	ErrCacheRequired = errors.New("imagekit: Cache is required")
)
