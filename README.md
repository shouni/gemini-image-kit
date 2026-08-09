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

`ImageRequest` 1 つで、単一参照画像からの生成も、複数参照画像を統合した 1 枚の画像生成もサポートします（`Images` の枚数が解釈を決めます）。漫画制作だけでなく、商品画像、広告素材、キャラクター差分、ゲームアセット、SNS クリエイティブなどの生成ワークフローに利用できます。既存画像の編集（構図を保った部分修正）が必要な場合は、既存画像を参照に、編集指示をプロンプトとして `Generate` を呼ぶことで、Gemini の会話型マルチモーダル画像モデル（Nano Banana系）による編集が行えます。

---

## ✨ 主な特徴 (Features)

* **🖼️ Unified Generator**:
  * `Generate` / `GenerateBatch` により、単一・複数参照画像の生成と一括生成を一貫して管理。
  * レート制限（`WithRateLimit`）・並列度（`WithMaxConcurrency`）・リクエストタイムアウト（`WithRequestTimeout`）を内蔵。利用側で errgroup + rate.Limiter を組む必要はありません。
* **🧩 Image Fusion Workflow**:
  * 複数の参照画像を Gemini の入力パーツとして収集し、プロンプトと組み合わせて1枚の画像を生成。
  * 参照画像の取得（GCS / HTTP）は**並行実行**。参照が増えても待ち時間が積み上がりません。結果の並び順は入力順のまま保たれます。
* **🔗 Hybrid Asset Workflow**:
  * Vertex AI モード: `gs://` スキームを検知し、GCS 上のデータを転送なしで Gemini に直接参照させることで、爆速な解析とリソース節約を実現。
  * Gemini API モード: Gemini File API (`files/xxxx`) を優先利用し、キャッシュがない場合は自動的にソースから取得して再アップロードするライフサイクル管理。**この判断はキット側が持つため、呼び出し側でアップロードを組む必要はありません**（下記「参照画像の解決方法」）。
  * 同一ソースへの同時アップロードは singleflight で1回にまとまります。同じ参照画像を並行して使っても File API 上に重複ファイルを作りません。
* **☁️ Intelligent MIME Prediction**:
  * GCS や外部 URI を参照するときは拡張子から `MIMEType` を推測し、インライン送信するときは実データの内容から判定します。
  * **推測できない拡張子では `MIMEType` を付けません**（サーバー側のコンテンツ判定に委ねます）。既定値を当てると PNG を JPEG と申告するような誤った型宣言になりうるためです。
* **🛡️ Fetch Policy Injection**:
  * 外部 URL 取得は `ports.Downloader` 経由に限定。SSRF 対策や許可ドメイン制御は、アプリケーション側で安全な Downloader を注入して適用します。
* **⚡️ Optimized Image Handling**:
  * **Stream-Based Upload**: File API へのアップロードは `bufio.Reader` を活用し、圧縮不要な場合はストリームで直接転送します（圧縮が必要な場合はメモリ上で再エンコードしてからアップロードします）。
  * **Selective Optimization**: PNG/GIF など圧縮対象の画像は JPEG に変換し、変換後の MIMEType も実データに合わせて送信します。
* **🧬 Robust Design**:
  * プロンプトとネガティブプロンプトの安全な結合、シード値の管理、アスペクト比の制御などを内蔵。
  * **シード自動採番が既定で有効**: シード未指定の生成でも `ImageResponse.UsedSeed` が実際に使われたシードを指すため、記録しておけば同じ結果を再現できます（`WithoutAutoSeed()` で無効化可）。
  * `ImageResponse` は `Model` / `Prompt` / `Usage`（トークン使用量）も返すため、コストや生成条件の記録に別途リクエストを持ち回る必要がありません。

---

## 📂 パッケージ構成 (Packages)

| パッケージ | 役割 |
| --- | --- |
| `github.com/shouni/gemini-image-kit/generator` | 画像生成の実装。`GeminiGenerator`（高レベル API）と `GeminiImageCore`（生成実行・参照画像の解決・File API のライフサイクル管理）。 |
| `github.com/shouni/gemini-image-kit/ports` | 公開インターフェースと入出力モデル。`ImageGenerator` / `BatchImageGenerator` / `ImageCacher` / `ContentReader` / `Downloader`、`ImageRequest` / `ImageResponse` / `GenerationOptions` / `ImageURI`。 |

`generator` は `ports` のインターフェースに対して実装されており、利用側は `ports` の型だけを参照して差し替えやモックができます。

---

## 🧭 Public API

```go
// 参照画像（0〜複数）と構成パラメータから1枚の画像を生成
Generate(ctx, ports.ImageRequest) (*ports.ImageResponse, error)

// 複数リクエストを、設定された並列度・レート制限の下で一括生成
// 一部が失敗しても成功した結果は破棄されない（失敗位置は nil + errors.Join）
GenerateBatch(ctx, []ports.ImageRequest) ([]*ports.ImageResponse, error)
```

生成時の任意設定は `generator.Option` で渡します。

```go
g, err := generator.NewGeminiGenerator(core,
    generator.WithRateLimit(30*time.Second, 1), // 発射間隔とバースト
    generator.WithMaxConcurrency(2),            // GenerateBatch の並列度
    generator.WithRequestTimeout(5*time.Minute))
```

利用側が依存するポートは `ports.ImageGenerator`（1 メソッド）です。参照解決やアップロードの内部分割はパッケージ内に閉じており、公開インターフェースではありません。

このライブラリの公開 API に `google.golang.org/genai` の型は現れません。生成 SDK の型は `go-gemini-client` の内側に閉じています。

### `ports.ImageResponse` の中身

| フィールド | 内容 |
| --- | --- |
| `Data` | 生成された画像のバイト列です。 |
| `MimeType` | 画像の MIME type。保存時の拡張子や Content-Type の決定に使います。 |
| `UsedSeed` | 送信したシード（既定では自動採番された値）。下記「シードと再現性」を参照。 |
| `Model` | 生成に使ったモデル名。リクエストを持ち回らずコスト集計や再現に使えます。 |
| `Prompt` | 実際に送信した最終プロンプト（ネガティブプロンプト結合済み）。 |
| `Usage` | トークン使用量（`*gemini.TokenUsage`）。モデルが返さない場合は nil です。 |

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

    "github.com/shouni/go-gemini-client/gemini"
    "github.com/shouni/gemini-image-kit/generator"
    "github.com/shouni/gemini-image-kit/ports"
)

func main() {
    ctx := context.Background()

    ai, err := gemini.NewClient(ctx, gemini.Config{
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
       log.Fatal(err)
    }

    if err := os.WriteFile("output.png", resp.Data, 0644); err != nil {
       log.Fatal(err)
    }
}
```

> 外部 URL を受け取る場合は、`ports.Downloader` 側で許可ドメイン、IP レンジ、タイムアウト、最大サイズなどを制御してください。`http.DefaultClient` は最小例です。

<details>
<summary>上の例で使っている補助実装（最小のプレースホルダ）</summary>

`Reader` / `HTTPClient` / `Cache` は注入する前提なので、動かすための最小実装を載せます。実運用では SSRF 対策済みの HTTP クライアント、GCS 読み取り、TTL 付きキャッシュ（`ttlcache` など）に置き換えてください。`Cache` は参照解決が並行に走るため、**同時アクセス安全な実装**である必要があります。

```go
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
```

</details>

### 2. 複数の参照画像を統合して生成する

```go
resp, err := g.Generate(ctx, ports.ImageRequest{
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
```

### 3. Vertex AI で GCS 画像を直接参照する

Vertex AI モードでは、`gs://` の参照画像はダウンロードせずに Gemini へ直接渡します。

```go
ai, err := gemini.NewClient(ctx, gemini.Config{
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

resp, err := g.Generate(ctx, ports.ImageRequest{
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
```

> Vertex AI では人物生成ポリシーを `GenerationOptions.PersonGeneration` で制御できます（`gemini.PersonGenerationAllowAll` / `AllowAdult` / `DontAllow`）。未指定時は `AllowAll` です。Gemini API バックエンドではこのフィールドは API の制約により常に無視されます。

### 4. 既存画像を編集する（Nano Banana系モデルによる会話型編集）

このライブラリに画像編集専用の API はありませんが、既存画像を `Images` に、編集指示を `Prompt` に渡して `Generate` を呼ぶことで、Gemini の会話型マルチモーダル画像モデル（`gemini-3.1-flash-image` など）による編集が行えます。

```go
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
    log.Fatal(err)
}

if err := os.WriteFile("edited.png", resp.Data, 0644); err != nil {
    log.Fatal(err)
}
```

> Vertex AI Imagen のマスクベース編集 API（`imagen-3.0-capability-001` 等）は2026年6月30日に廃止され、後継の「capability」モデルも用意されていません。マスクで領域を明示した編集が必要な場合は、この会話型編集では対応できない点に注意してください。

---

## ⚙️ 設定 (`generator.GeminiImageCoreConfig`)

| 設定項目 | 役割 | 既定値 |
| --- | --- | --- |
| `AIClient` | `gemini.Model`（生成・File API・バックエンド判定）。**必須** | - |
| `Reader` | `gs://` を読む `ports.ContentReader`。**必須** | - |
| `HTTPClient` | http(s) を読む `ports.Downloader`。**必須** | - |
| `Cache` | アップロード済みファイルの参照を保持する `ports.ImageCacher`（`Get`/`Set`）。**必須**（アップロードの使い回しがこのキットの主要なコスト最適化のため） | - |
| `CacheTTL` | 上記キャッシュの有効期間。**0 は補完せずそのまま渡します**（`ttlcache` では 0 が `DefaultTTL` そのもので、「キャッシュ側の既定に従う」という意味を持つため）。File API の保持期限より短く設定してください | 実装依存 |
| `Compress` | PNG/GIF を送信前に JPEG へ再圧縮するか | `false` |
| `CompressionQuality` | `Compress` が true のときの JPEG 品質 | `75`（`DefaultCompressionQuality`） |
| `UploadTimeout` | アップロード1回あたりの制限時間。共有実行は呼び出し元の context から切り離されるため、これが唯一の打ち切り手段です | `2m`（`DefaultUploadTimeout`） |
| `InlineReferences` | 参照画像を File API へ上げず常にインライン送信するか（使い捨ての参照向け） | `false` |
| `FetchTimeout` | 参照画像の取得（インライン送信経路）1回あたりの制限時間 | `1m`（`DefaultFetchTimeout`） |
| `MaxReferenceBytes` | 参照画像1枚あたりのサイズ上限。超えるとエラー | `32MiB`（`DefaultMaxReferenceBytes`） |
| `Logger` | ライブラリ内部ログの出力先（`*slog.Logger`） | `slog.Default()` |

必須依存が欠けている場合は `ErrAIClientRequired` / `ErrReaderRequired` / `ErrHTTPClientRequired` / `ErrCacheRequired` を返します。

---

## 🎯 参照画像の解決方法

`ImageURI` 1件をどう送るかは、バックエンドと URI の種類でキットが決めます。

| 条件 | 解決方法 |
| --- | --- |
| Vertex AI + `gs://` | 転送せず直接参照（最も安い。`FileAPIURI` の指定より優先） |
| `FileAPIURI` が指定済み | その URI をそのまま参照 |
| Gemini API | **File API へアップロードして URI 参照**（キャッシュ + singleflight） |
| Vertex AI + `gs://` 以外 | インライン送信（Vertex AI に File API は無いため） |

Gemini API バックエンドで同じ参照画像を繰り返し使う場合、毎回バイト列を送るより安く済みます。逆に参照画像が毎回異なる使い捨てのワークロードでは、アップロードの往復と File API 上のファイルが無駄になるため、`GeminiImageCoreConfig.InlineReferences: true` で常にインライン送信へ固定できます。

アップロードに失敗した場合は警告ログを出してインライン送信にフォールバックします（アップロードは送信量を減らすための最適化なので、その失敗で生成自体を落としません）。ただし失敗の原因が呼び出し側のキャンセルである場合はフォールバックせず、キャンセルをそのまま返します。

File API 上のファイルには保持期限があるため、`CacheTTL` はそれより短く設定してください。

### 参照画像の取得は並行

`Generate` に複数の参照画像を渡した場合、GCS / HTTP からの取得は並行に走ります。そのため注入する `ports.ImageCacher` は**同時アクセス安全**である必要があります（`ttlcache` などロック付きの実装、または自前でロック）。取得が失敗した場合は、入力順で最初に失敗した参照のエラーが返ります（実行ごとにエラーが変わらないようにするため）。

---

## 🎲 シードと再現性

`ImageResponse.UsedSeed` は「生成に使われたシード」ですが、**API はレスポンスにシードを返しません**。そのため API 側にシード選択を委ねると値は知りようがなく、`UsedSeed` は 0 のままになります（0 は有効なシードなので「未記録」と区別も付きません）。

このため**シード未指定のリクエストには既定で生成側がシードを採番してから送信します**。`UsedSeed` は常に実際の値を指し、記録しておけば同じ結果を再現できます。生成結果のランダム性は変わりません（シードを選ぶのが API 側か生成側かの違いです）。シード管理を完全に呼び出し側で行う場合のみ `WithoutAutoSeed()` を使ってください。

---

## 📜 エラーハンドリング

`generator` パッケージは以下のセンチネルエラーをエクスポートしています。`errors.Is` で判定できます。

- `ErrModelRequired`: 生成リクエストにモデル名が指定されていない場合。
- `ErrEmptyPrompt`: プロンプト（ネガティブプロンプト含む）が空の場合。
- `ErrUnsupportedFileFormat`: 取得したデータが画像として扱えない場合。
- `ErrReferenceTooLarge`: 参照画像が `MaxReferenceBytes` を超えている場合。
- `ErrNoImageData`: レスポンスに画像データが含まれていない場合。

コンストラクタの必須依存に対するセンチネルは次のとおりです。

- `ErrAIClientRequired` / `ErrReaderRequired` / `ErrHTTPClientRequired` / `ErrCacheRequired`: `NewGeminiImageCore` に対応する依存が渡されなかった場合。
- `ErrExecutorRequired`: `NewGeminiGenerator` に core が渡されなかった場合。

---

## 🤝 依存関係 (Dependencies)

* [shouni/go-gemini-client](https://github.com/shouni/go-gemini-client) - **Backend（Vertex AI / Google AI）を抽象化するクライアント**
* [google.golang.org/genai](https://pkg.go.dev/google.golang.org/genai) - Google Gemini 公式 SDK（`go-gemini-client` 経由の間接依存。公開 API には現れません）

---

## 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
