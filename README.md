# 🎨 Gemini Image Kit

[![CI](https://github.com/shouni/gemini-image-kit/actions/workflows/ci.yml/badge.svg)](https://github.com/shouni/gemini-image-kit/actions/workflows/ci.yml)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/gemini-image-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/gemini-image-kit)](https://github.com/shouni/gemini-image-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/gemini-image-kit.svg)](https://pkg.go.dev/github.com/shouni/gemini-image-kit)
[![Status](https://img.shields.io/badge/Status-Completed-brightgreen)](#)

## 🚀 概要 (About) - アセット運用を最適化する画像生成コア

**Gemini Image Kit** は、Google Gemini API を利用した画像生成を、Go言語でより直感的、かつ堅牢に実装するためのツールキットです。

単なる API ラッパーではなく、「**GCS/外部URLからの参照画像自動取得**」「**Gemini File API とキャッシュの一貫性管理**」「**注入可能な Downloader による取得ポリシー制御**」「**インメモリ画像圧縮**」といった、実用的なアプリケーション開発で直面する課題を解決するために設計されています。

`SingleImageRequest` による単一参照画像からの生成、`ImageFusionRequest` による複数参照画像を統合した1枚の画像生成をサポートします。漫画制作だけでなく、商品画像、広告素材、キャラクター差分、ゲームアセット、SNS クリエイティブなどの生成ワークフローに利用できます。既存画像の編集（構図を保った部分修正）が必要な場合は、`GenerateSingleImage` に既存画像を入力として渡し、編集指示をプロンプトとして渡すことで、Gemini の会話型マルチモーダル画像モデル（Nano Banana系）による編集が行えます。

---

## ✨ 主な特徴 (Features)

* **🖼️ Unified Generator**:
  * `GenerateSingleImage`、`GenerateFusedImage` により、単一参照画像の生成、複数参照画像の統合生成を一貫して管理。
* **🧩 Image Fusion Workflow**:
  * 複数の参照画像を Gemini の入力パーツとして収集し、プロンプトと組み合わせて1枚の画像を生成。
  * 参照画像の取得（GCS / HTTP）は**並行実行**。参照が増えても待ち時間が積み上がりません。結果の並び順は入力順のまま保たれます。
* **🔗 Hybrid Asset Workflow**:
  * Vertex AI モード: `gs://` スキームを検知し、GCS 上のデータを転送なしで Gemini に直接参照させることで、爆速な解析とリソース節約を実現。
  * Gemini API モード: Gemini File API (`files/xxxx`) を優先利用し、キャッシュがない場合は自動的にソースから取得して再アップロードするライフサイクル管理。
* **☁️ Intelligent MIME Prediction**:
  * GCS や外部 URI からの参照時、拡張子に基づいて `MIMEType` を自動推測。SDK の `Required` 制約を透過的に解決します。
* **🛡️ Fetch Policy Injection**:
  * 外部 URL 取得は `ports.Downloader` 経由に限定。SSRF 対策や許可ドメイン制御は、アプリケーション側で安全な Downloader を注入して適用します。
* **⚡️ Optimized Image Handling**:
  * **Stream-Based Upload**: File API へのアップロードは `bufio.Reader` を活用し、圧縮不要な場合はストリームで直接転送します（圧縮が必要な場合はメモリ上で再エンコードしてからアップロードします）。
  * **Selective Optimization**: PNG/GIF など圧縮対象の画像は JPEG に変換し、変換後の MIMEType も実データに合わせて送信します。
* **🧬 Robust Design**:
  * プロンプトとネガティブプロンプトの安全な結合、シード値の管理、アスペクト比の制御などを内蔵。
  * `WithAutoSeed()` を付けると、シード未指定の生成でも `ImageResponse.UsedSeed` が**実際に使われたシード**を返すため、記録しておけば同じ結果を再現できます。

---

## 🧭 Public API

```go
// 1枚の参照画像を使って画像を生成
GenerateSingleImage(ctx, ports.SingleImageRequest)

// 複数の参照画像を統合して1枚の画像を生成
GenerateFusedImage(ctx, ports.ImageFusionRequest)
```

内部の `GeminiImageCore` は `ports.AssetManager`（File API のアップロード・削除）と `ports.ImageExecutor`（生成実行・参照画像の準備）を担います。参照画像は `gemini.Attachment` として渡され、バイト列でも `gs://` や `files/...` の URI 参照でも同じ型で表現されます。

```go
// 生成リクエストの実行（プロンプト + 参照画像の添付）
ExecuteRequest(ctx, model string, prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions)

// URL / GCS から参照画像を取得し、添付へ変換（キャッシュ済みなら URI 参照を返す）
PrepareImageAttachment(ctx, rawURL string) (gemini.Attachment, error)
```

このライブラリの公開 API に `google.golang.org/genai` の型は現れません。生成 SDK の型は `go-gemini-client` の内側に閉じています。

### シードと再現性

`ImageResponse.UsedSeed` は「生成に使われたシード」ですが、**API はレスポンスにシードを返しません**。そのためリクエストで `Seed` を指定しなかった場合、既定では API 側がランダムに選んだ値は知りようがなく、`UsedSeed` は 0 のままになります。これを「使われたシード」として記録すると、再生成時に 0 という別のシードを使うことになります。

`WithAutoSeed()` を付けると、シード未指定のリクエストに対して生成側でシードを決めてから送信するため、`UsedSeed` が常に実際の値を指します。生成結果のランダム性は変わりません（シードを選ぶのが API 側か生成側かの違いです）。

```go
generator, err := generator.NewGeminiGenerator(core, generator.WithAutoSeed())
```

### 参照画像の取得は並行

`GenerateFusedImage` に複数の参照画像を渡した場合、GCS / HTTP からの取得は並行に走ります。そのため注入する `ports.ImageCacher` は**同時アクセス安全**である必要があります（`go-cache` などロック付きの実装、または自前でロック）。取得が失敗した場合は、入力順で最初に失敗した参照のエラーが返ります（実行ごとにエラーが変わらないようにするため）。

---

## 🚀 Quick Start

### 1. Gemini API で単一参照画像から生成する

```go
package main

import (
    "context"
    "errors"
    "io"
    "log"
    "net/http"
    "os"
    "sync"
    "time"

    client "github.com/shouni/go-gemini-client/gemini"
    "github.com/shouni/gemini-image-kit/generator"
    "github.com/shouni/gemini-image-kit/ports"
)

func main() {
    ctx := context.Background()

    ai, err := client.NewClient(ctx, client.Config{
       APIKey: os.Getenv("GEMINI_API_KEY"),
    })
    if err != nil {
       log.Fatal(err)
    }

    core, err := generator.NewGeminiImageCore(generator.GeminiImageCoreConfig{
       AIClient:   ai,
       Reader:     noStorageReader{},
       HTTPClient: httpDownloader{client: http.DefaultClient},
       Cache:      newMemoryCache(),
       CacheTTL:   24 * time.Hour,
       Compress:   true, // PNG/GIF を JPEG に変換して送信サイズを抑える
    })
    if err != nil {
       log.Fatal(err)
    }

    g, err := generator.NewGeminiGenerator(core)
    if err != nil {
       log.Fatal(err)
    }

    resp, err := g.GenerateSingleImage(ctx, ports.SingleImageRequest{
       GenerationOptions: ports.GenerationOptions{
          Model:          "gemini-3-pro-image",
          Prompt:         "参照画像の人物を、白背景の商品広告風ポートレートにしてください。",
          NegativePrompt: "low quality, blurry, distorted hands",
          AspectRatio:    "1:1",
          ImageSize:      "1K",
       },
       Image: ports.ImageURI{
          ReferenceURL: "https://example.com/reference.png",
       },
    })
    if err != nil {
       log.Fatal(err)
    }

    if err := os.WriteFile("output.png", resp.Data, 0644); err != nil {
       log.Fatal(err)
    }
}

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
       resp.Body.Close()
       return nil, errors.New("failed to download image")
    }
    return resp.Body, nil
}

type noStorageReader struct{}

func (noStorageReader) Open(ctx context.Context, uri string) (io.ReadCloser, error) {
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

func (c *memoryCache) Set(key string, value any, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.data[key] = value
}

func (c *memoryCache) Delete(key string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    delete(c.data, key)
}
```

> 外部 URL を受け取る場合は、`ports.Downloader` 側で許可ドメイン、IP レンジ、タイムアウト、最大サイズなどを制御してください。`http.DefaultClient` は最小例です。

### 2. 複数の参照画像を統合して生成する

```go
resp, err := g.GenerateFusedImage(ctx, ports.ImageFusionRequest{
    GenerationOptions: ports.GenerationOptions{
       Model:       "gemini-3-pro-image",
       Prompt:      "1枚目のキャラクターを、2枚目の服装と3枚目の背景に自然に合成してください。",
       AspectRatio: "16:9",
       ImageSize:   "2K",
    },
    Images: []ports.ImageURI{
       {ReferenceURL: "https://example.com/character.png"},
       {ReferenceURL: "https://example.com/outfit.png"},
       {ReferenceURL: "https://example.com/background.png"},
    },
})
```

### 3. Vertex AI で GCS 画像を直接参照する

Vertex AI モードでは、`gs://` の参照画像はダウンロードせずに Gemini へ直接渡します。

```go
ai, err := client.NewClient(ctx, client.Config{
    ProjectID:  "your-google-cloud-project-id",
    LocationID: "asia-northeast1",
})
if err != nil {
    log.Fatal(err)
}

core, err := generator.NewGeminiImageCore(generator.GeminiImageCoreConfig{
	AIClient:   ai,
	Reader:     noStorageReader{},
	HTTPClient: httpDownloader{client: http.DefaultClient},
	Cache:      newMemoryCache(),
	CacheTTL:   24*time.Hour,
	Compress:   false,
})
if err != nil {
    log.Fatal(err)
}

g, err := generator.NewGeminiGenerator(core)
if err != nil {
    log.Fatal(err)
}

resp, err := g.GenerateSingleImage(ctx, ports.SingleImageRequest{
    GenerationOptions: ports.GenerationOptions{
       Model:       "gemini-3-pro-image",
       Prompt:      "この商品画像を、SNS 広告向けの高級感ある構図にしてください。",
       AspectRatio: "4:5",
       ImageSize:   "1K",
    },
    Image: ports.ImageURI{
       ReferenceURL: "gs://your-bucket/products/source.png",
    },
})
```

> Vertex AI では人物生成ポリシーを `GenerationOptions.PersonGeneration` で制御できます（`gemini.PersonGenerationAllowAll` / `AllowAdult` / `DontAllow`）。未指定時は `AllowAll` です。Gemini API バックエンドではこのフィールドは API の制約により常に無視されます。

### 4. 既存画像を編集する（Nano Banana系モデルによる会話型編集）

このライブラリに画像編集専用の API はありませんが、既存画像を `SingleImageRequest.Image` に、編集指示を `Prompt` に渡して `GenerateSingleImage` を呼ぶことで、Gemini の会話型マルチモーダル画像モデル（`gemini-3.1-flash-image` など）による編集が行えます。

```go
resp, err := g.GenerateSingleImage(ctx, ports.SingleImageRequest{
    GenerationOptions: ports.GenerationOptions{
       Model:  "gemini-3.1-flash-image",
       Prompt: "対象領域のバッグを黒いレザーバッグに差し替えてください。他の部分は変更しないでください。",
    },
    Image: ports.ImageURI{
       ReferenceURL: "gs://your-bucket/edit/source.png",
    },
})
if err != nil {
    log.Fatal(err)
}

if err := os.WriteFile("edited.png", resp.Data, 0644); err != nil {
    log.Fatal(err)
}
```

> Vertex AI Imagen のマスクベース編集 API（`imagen-3.0-capability-001` 等）は2026年6月30日に廃止され、後継の「capability」モデルも用意されていません。マスクで領域を明示した編集が必要な場合は、この会話型編集では対応できない点に注意してください。

---

## 📜 エラーハンドリング

`generator` パッケージは以下のセンチネルエラーをエクスポートしています。`errors.Is` で判定できます。

- `ErrModelRequired`: 生成リクエストにモデル名が指定されていない場合。
- `ErrEmptyPrompt`: プロンプト（ネガティブプロンプト含む）が空の場合。
- `ErrUnsupportedFileFormat`: 取得したデータが画像として扱えない場合。
- `ErrFileNotInCache`: File API のファイル名がキャッシュから引けず削除できない場合。
- `ErrNoImageData`: レスポンスに画像データが含まれていない場合。

---

## 📂 パッケージ構成 (Packages)

| パッケージ | 役割 |
| --- | --- |
| `github.com/shouni/gemini-image-kit/generator` | 画像生成の実装。`GeminiGenerator`（高レベル API）と `GeminiImageCore`（生成実行・参照画像の準備・File API のライフサイクル管理）。 |
| `github.com/shouni/gemini-image-kit/ports` | 公開インターフェースと入出力モデル。`ImageGenerator` / `ImageExecutor` / `AssetManager` / `ImageCacher`、リクエスト・レスポンス型、`ImageURI`。 |
| `github.com/shouni/gemini-image-kit/imgutil` | 画像ユーティリティ。MIME タイプ判定と送信前の JPEG 圧縮。 |

`generator` は `ports` のインターフェースに対して実装されており、利用側は `ports` の型だけを参照して差し替えやモックができます。

---

## 🤝 依存関係 (Dependencies)

* [shouni/go-gemini-client](https://github.com/shouni/go-gemini-client) - **Backend（Vertex AI / Google AI）を抽象化するクライアント**
* [google.golang.org/genai](https://pkg.go.dev/google.golang.org/genai) - Google Gemini 公式 SDK（`go-gemini-client` 経由の間接依存。公開 API には現れません）

---

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
