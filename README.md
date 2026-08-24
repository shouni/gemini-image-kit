# 🎨 Gemini Image Kit

[![CI](https://github.com/shouni/gemini-image-kit/actions/workflows/ci.yml/badge.svg)](https://github.com/shouni/gemini-image-kit/actions/workflows/ci.yml)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/gemini-image-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/gemini-image-kit)](https://github.com/shouni/gemini-image-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/gemini-image-kit.svg)](https://pkg.go.dev/github.com/shouni/gemini-image-kit)
[![Status](https://img.shields.io/badge/Status-Active-brightgreen)](#)

## 🚀 概要 (About) - アセット運用を最適化する画像生成コア

**Gemini Image Kit** は、Google Gemini API を利用した画像生成を、Go言語でより直感的、かつ堅牢に実装するためのツールキットです。

単なる API ラッパーではなく、生の SDK に無いものを足します。**参照画像をどう送るかを差し替えられる仕組み**（`gs://` を転送せず直接参照 / GCS・外部 URL から取得 / File API へ上げてキャッシュ）、インメモリ画像圧縮、リクエスト単位のレート制限・並列度・タイムアウトです。

`ImageRequest` 1 つで、参照なしのテキスト生成も、単一参照からの生成も、複数参照を統合した融合生成も表現できます（`Images` の枚数が解釈を決めます）。漫画制作だけでなく、商品画像、広告素材、キャラクター差分、ゲームアセット、SNS クリエイティブなどの生成ワークフローに利用できます。既存画像の編集も、編集対象を参照に、編集指示をプロンプトとして `Generate` を呼ぶだけです。

---

## ✨ 主な特徴 (Features)

* **🖼️ Unified Generator**:
  * `Generate` / `GenerateBatch` により、単一・複数参照画像の生成と一括生成を一貫して管理。
  * レート制限（`WithRateLimit`）・並列度（`WithMaxConcurrency`）・リクエストタイムアウト（`WithRequestTimeout`）を内蔵。利用側で errgroup + rate.Limiter を組む必要はありません。
* **🔗 Pluggable Reference Resolution**:
  * 参照画像の送り方を `ports.ReferenceResolver` として**アプリ側が選びます**。`gs://` の直接参照（`GCSResolver`、依存ゼロ）、取得してインライン（`FetchResolver`）、File API へ上げてキャッシュ（`FileAPIResolver`）を `ResolverChain` で組み合わせます（下記「参照画像の解決方法」）。
  * 依存は選んだ resolver だけが要求します。`gs://` しか使わない構成なら、取得・キャッシュの実装を一切渡す必要がありません。
  * File API 経路では、同一ソースへの同時アップロードが singleflight で1回にまとまります。同じ参照画像を並行して使っても File API 上に重複ファイルを作りません。
* **🧩 Image Fusion Workflow**:
  * 複数の参照画像を収集し、プロンプトと組み合わせて1枚の画像を生成。
  * 参照の解決は**並行実行**。GCS / HTTP の往復を伴う経路でも、参照が増えて待ち時間が積み上がりません。結果の並び順は入力順のまま保たれます。
* **☁️ Intelligent MIME Prediction**:
  * URI 参照では拡張子から `MIMEType` を推測し、インライン送信では実データの内容から判定します。
  * **推測できない拡張子では `MIMEType` を付けません**（サーバー側のコンテンツ判定に委ねます）。既定値を当てると PNG を JPEG と申告するような誤った型宣言になりうるためです。
* **🛡️ Fetch Policy Injection**:
  * 外部 URL 取得は `ports.Downloader` 経由に限定。SSRF 対策や許可ドメイン制御は、アプリケーション側で安全な Downloader を注入して適用します。
* **⚡️ Optimized Image Handling**:
  * **Selective Optimization**: `Compress` を有効にすると PNG/GIF は JPEG へ再圧縮して送信サイズを抑えます。変換後の `MIMEType` も実データに合わせて送信します。
  * 圧縮対象でない形式はデコードせず、取得したストリームをそのままアップロードへ渡します。
* **🧬 Robust Design**:
  * プロンプトとネガティブプロンプトの安全な結合を内蔵。アスペクト比などの生成パラメータは `gemini.GenerateOptions` の埋め込みでそのまま指定できます。
  * **シード自動採番が既定で有効**（下記「シードと再現性」）。
  * `ImageResponse` は画像バイト列に加えて `Model` / `Prompt` / `Usage` も返すため、コストや生成条件の記録にリクエストを持ち回る必要がありません。

---

## 📂 パッケージ構成 (Packages)

| パッケージ | 役割 |
| --- | --- |
| `github.com/shouni/gemini-image-kit/generator` | 画像生成の実装（`Generator`）と、参照画像の解決を担う resolver 群（`GCSResolver` / `FetchResolver` / `FileAPIResolver` / `ResolverChain`）。 |
| `github.com/shouni/gemini-image-kit/ports` | 公開インターフェースと入出力モデル。`ImageGenerator` / `BatchImageGenerator` / `ReferenceResolver` / `ImageCacher` / `ContentReader` / `Downloader`、`ImageRequest` / `ImageResponse` / `GenerationOptions` / `ImageURI`。応答を保存するための小さなヘルパー `ExtensionByMIMEType` もここにあります。 |

`generator` は `ports` のインターフェースに対して実装されており、利用側は `ports` の型だけを参照して差し替えやモックができます。

---

## 🧭 Public API

```go
// 参照画像（0〜複数）と構成パラメータから1枚の画像を生成
Generate(ctx, ports.ImageRequest) (*ports.ImageResponse, error)

// 複数リクエストを、設定された並列度・レート制限の下で一括生成
// 一部が失敗しても成功した結果は破棄されない
// （失敗位置は nil、エラーは requests[i] の添字付きで errors.Join）
GenerateBatch(ctx, []ports.ImageRequest) ([]*ports.ImageResponse, error)
```

生成時の任意設定は `generator.Option` で渡します。

```go
g, err := generator.New(client, resolver,
    generator.WithRateLimit(30*time.Second, 1), // 発射間隔とバースト
    generator.WithMaxConcurrency(2),            // GenerateBatch の並列度
    generator.WithRequestTimeout(5*time.Minute))
```

第 2 引数の `resolver` は**必須**です。参照画像をどう送るか（`gs://` を直接参照 / File API へ上げて使い回す / 取得してインライン）は運用上の判断なので、キットが既定で黙って選ぶことはしません。詳細は「参照画像の解決方法」を参照してください。

利用側が依存するポートは `ports.ImageGenerator`（1 メソッド）です。

このライブラリの公開 API に `google.golang.org/genai` の型は現れません。生成 SDK の型は `go-gemini-client` の内側に閉じています。

### `ports.ImageResponse` の中身

| フィールド | 内容 |
| --- | --- |
| `Data` | 生成された画像のバイト列です。 |
| `MimeType` | 画像の MIME type。Content-Type にそのまま使えます。保存時の拡張子は `ports.ExtensionByMIMEType` で引けます（下記）。 |
| `UsedSeed` | 送信したシード（既定では自動採番された値）。下記「シードと再現性」を参照。 |
| `Model` | 生成に使ったモデル名。リクエストを持ち回らずコスト集計や再現に使えます。 |
| `Prompt` | 実際に送信した最終プロンプト（ネガティブプロンプト結合済み）。 |
| `Usage` | トークン使用量（`*gemini.TokenUsage`）。モデルが返さない場合は nil です。 |

### 保存ファイルの拡張子 (`ports.ExtensionByMIMEType`)

```go
path := fmt.Sprintf("keyframe%s", ports.ExtensionByMIMEType(resp.MimeType)) // => keyframe.jpg
```

`image/jpeg` → `.jpg`、`image/png` → `.png`、`image/webp` → `.webp`、`image/gif` → `.gif` を返します。
`Content-Type` ヘッダーの値をそのまま渡せます（`image/jpeg; charset=binary` のようなパラメーター付きでも
メディアタイプだけを見ます）。正式な MIME type ではない `image/jpg` も、生成モデルの応答に実際に現れるため
`.jpg` として扱います。判定できない MIME type には `.png` を返すので、保存そのものが止まることはありません
（誤った拡張子の実害は保存パスの見た目に留まり、配信時の Content-Type は `MimeType` から別途付くためです）。

名前と役割は標準の `mime.ExtensionsByType` に合わせてあります。標準版を使わないのは、OS の MIME データベース
次第で返る拡張子が変わるためです（`image/jpeg` に `.jpe` が返る環境があります）。保存パスは URL や履歴に
残り続けるので、対応表を固定しています。

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

    // Gemini API バックエンド: File API へ上げて使い回し、失敗したら取得してインライン
    upload, err := generator.NewFileAPIResolver(generator.FileAPIResolverConfig{
       Files:      ai,
       Reader:     noStorageReader{},
       Downloader: httpDownloader{client: http.DefaultClient},
       Cache:      newMemoryCache(),
       CacheTTL:   24 * time.Hour,
       Compress:   true, // PNG/GIF を JPEG に変換して送信サイズを抑える
    })
    if err != nil {
       log.Fatal(err)
    }
    inline, err := generator.NewFetchResolver(generator.FetchResolverConfig{
       Reader:     noStorageReader{},
       Downloader: httpDownloader{client: http.DefaultClient},
       Compress:   true,
    })
    if err != nil {
       log.Fatal(err)
    }

    g, err := generator.New(ai, generator.NewResolverChain(upload, inline))
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

`Reader` / `Downloader` / `Cache` は注入する前提なので、動かすための最小実装を載せます。実運用では SSRF 対策済みの HTTP クライアント、GCS 読み取り、TTL 付きキャッシュ（`ttlcache` など）に置き換えてください。`Cache` は参照解決が並行に走るため、**同時アクセス安全な実装**である必要があります。

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

`GCSResolver` を経路に置くと、`gs://` の参照画像はダウンロードされずそのまま Vertex AI へ渡ります。

```go
ai, err := gemini.NewClient(ctx, gemini.Config{
    ProjectID:  "your-google-cloud-project-id",
    LocationID: "asia-northeast1",
})
if err != nil {
    log.Fatal(err)
}

// Vertex AI バックエンド: gs:// は転送せず直接参照。それ以外は取得してインライン。
// 参照が gs:// だけで済むなら NewGCSResolver() 単体で足り、取得系の依存は不要です。
inline, err := generator.NewFetchResolver(generator.FetchResolverConfig{
	Reader:     noStorageReader{},
	Downloader: httpDownloader{client: http.DefaultClient},
})
if err != nil {
    log.Fatal(err)
}

g, err := generator.New(ai, generator.NewResolverChain(generator.NewGCSResolver(), inline))
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

## ⚙️ 設定

### `generator.New(client, resolver, opts...)`

| 引数 | 役割 |
| --- | --- |
| `client` | `gemini.Generator`（`GenerateWithAttachments` の 1 メソッド）。**必須**。`gemini.BackendInspector` も満たす実クライアントを渡すと、安全設定と人物生成の既定値がバックエンドに応じて切り替わります |
| `resolver` | `ports.ReferenceResolver`。**必須**（既定値なし） |
| `opts` | `WithRateLimit` / `WithMaxConcurrency` / `WithRequestTimeout` / `WithoutAutoSeed` |

### `generator.FetchResolverConfig`（取得してインライン送信）

| 設定項目 | 役割 | 既定値 |
| --- | --- | --- |
| `Reader` | `gs://` を読む `ports.ContentReader`。**必須** | - |
| `Downloader` | http(s) を読む `ports.Downloader`。**必須**。呼び出し側が入力した URL がそのまま渡るため、**SSRF 対策とドメイン許可リストは呼び出し側の責務**です | - |
| `FetchTimeout` | 取得1回あたりの制限時間 | `1m`（`DefaultFetchTimeout`） |
| `MaxReferenceBytes` | 参照画像1枚あたりのサイズ上限。超えるとエラー | `32MiB`（`DefaultMaxReferenceBytes`） |
| `Compress` | PNG/GIF を送信前に JPEG へ再圧縮するか | `false` |
| `CompressionQuality` | `Compress` が true のときの JPEG 品質 | `75`（`DefaultCompressionQuality`） |

### `generator.FileAPIResolverConfig`（File API へアップロードして URI 参照）

Gemini API バックエンド専用です（Vertex AI に File API はありません）。

| 設定項目 | 役割 | 既定値 |
| --- | --- | --- |
| `Files` | `gemini.FileManager`（アップロード先）。**必須** | - |
| `Reader` / `Downloader` | アップロード元の取得に使います。**必須** | - |
| `Cache` | アップロード済み URI を保持する `ports.ImageCacher`（`Get`/`Set`）。**必須**（使い回しがこの resolver の存在理由のため） | - |
| `CacheTTL` | 上記キャッシュの有効期間。**0 は補完せずそのまま渡します**（`ttlcache` では 0 が `DefaultTTL` そのもので、「キャッシュ側の既定に従う」という意味を持つため）。File API の保持期限より短く設定してください | 実装依存 |
| `UploadTimeout` | アップロード1回あたりの制限時間。共有実行は呼び出し元の context から切り離されるため、これが唯一の打ち切り手段です | `2m`（`DefaultUploadTimeout`） |
| `FetchTimeout` / `MaxReferenceBytes` | 取得側の上限（`FetchResolverConfig` と同じ） | 同上 |
| `Compress` / `CompressionQuality` | アップロード前の再圧縮 | `false` / `75` |
| `Logger` | ライブラリ内部ログの出力先（`*slog.Logger`） | `slog.Default()` |

---

## 🎯 参照画像の解決方法

`ImageURI` 1 件をどう送るかは、注入した `ports.ReferenceResolver` が決めます。キットは 3 つの実装と、それらを並べる `ResolverChain` を提供します。

| resolver | 解決方法 | 依存 | 制約 |
| --- | --- | --- | --- |
| `NewGCSResolver()` | `gs://` を転送せず直接参照（最も安い） | **なし** | Vertex AI 専用（`New` が強制） |
| `NewFetchResolver(cfg)` | 取得してバイト列をインライン送信 | Reader / Downloader | なし |
| `NewFileAPIResolver(cfg)` | File API へアップロードして URI 参照（キャッシュ + singleflight）。`ImageURI.FileAPIURI` が設定済みならアップロードを省いてそれを使う | Files / Reader / Downloader / Cache | Gemini API 専用（`New` が強制） |

`ResolverChain` は先頭から順に試し、`ports.ErrResolverNotApplicable`（管轄外）を返した resolver だけ読み飛ばします。取得失敗のような実エラーはその場で返します — 次へ流すと、ネットワーク障害が「参照を解決できません」にすり替わって原因が消えるためです。誰も扱えなければ `ErrUnresolvedReference` になります（経路の設定漏れ）。

```go
// Vertex AI: gs:// は直接参照、それ以外は取得してインライン
generator.NewResolverChain(generator.NewGCSResolver(), fetchResolver)

// Gemini API: File API へ上げて使い回し、失敗したら取得してインライン
generator.NewResolverChain(fileAPIResolver, fetchResolver)

// 参照が gs:// だけの構成（取得もキャッシュも不要）
generator.NewGCSResolver()
```

`FileAPIResolver` はアップロードに失敗すると警告ログを出して**辞退**し、次の resolver に委ねます（アップロードは送信量を減らすための最適化なので、その失敗で生成自体を落としません）。ただし失敗の原因が呼び出し側のキャンセルである場合は辞退せず、キャンセルをそのまま返します。

バックエンド制約のある resolver をもう一方のクライアントと組み合わせると、`generator.New` が**構築時に**弾きます。生成時まで気付けないと、どちらも見つけにくい壊れ方をするためです。

| 組み合わせ | エラー | 弾かないとどうなるか |
| --- | --- | --- |
| `GCSResolver` + Gemini API | `ErrVertexAIRequired` | Gemini API は `gs://` を解決できず、参照が一切効かない |
| `FileAPIResolver` + Vertex AI | `ErrGeminiAPIRequired` | Vertex に File API が無いのでアップロードが必ず失敗し、毎回インラインへ落ちる。**生成は成功する**ぶん気付きにくく、`gs://` を 2 回ダウンロードし続ける |

判定できるのは `gemini.BackendInspector` を満たすクライアントだけで、バックエンドを申告しないクライアント（テスト用フェイクなど）は素通しします。

File API 上のファイルには保持期限があるため、`CacheTTL` はそれより短く設定してください。

### 参照画像の解決は並行

`Generate` に複数の参照画像を渡した場合、解決は並行に走ります。そのため注入する `ports.ReferenceResolver` は**同時アクセス安全**である必要があります（`FileAPIResolver` を使う場合は、それが持つ `ports.ImageCacher` も同様。`ttlcache` などロック付きの実装、または自前でロック）。解決が失敗した場合は、入力順で最初に失敗した参照のエラーが返ります（実行ごとにエラーが変わらないようにするため）。

---

## 🎲 シードと再現性

`ImageResponse.UsedSeed` は名前こそ「生成に使われたシード」ですが、中身は**送信したシードの記録**です。**API はレスポンスにシードを返さない**ため、シードの選択を API 側に委ねてしまうと、実際に使われた値は知りようがなく、`UsedSeed` は 0 のままになってしまいます（0 は有効なシードなので、「未記録」と区別することもできません）。

そのため**シード未指定のリクエストには、既定で生成側がシードを採番してから送信します**。`UsedSeed` は常に実際に送信した値を指し、記録しておけば同じ結果を再現できます。生成結果のランダム性は変わりません（シードを選ぶのが API 側か生成側かの違いです）。シード管理を完全に呼び出し側で行う場合のみ `WithoutAutoSeed()` を使ってください（このとき、シードを指定しなければ `UsedSeed` は 0 になります）。

---

## 📜 エラーハンドリング

`generator` パッケージは以下のセンチネルエラーをエクスポートしています。`errors.Is` で判定できます。

- `ErrModelRequired`: 生成リクエストにモデル名が指定されていない場合。
- `ErrEmptyPrompt`: プロンプト（ネガティブプロンプト含む）が空の場合。
- `ErrUnsupportedFileFormat`: 取得したデータが画像として扱えない場合。
- `ErrReferenceTooLarge`: 参照画像が `MaxReferenceBytes` を超えている場合。
- `ErrNoImageData`: レスポンスに画像データが含まれていない場合。
- `ErrUnresolvedReference`: どの resolver も参照を扱えなかった場合（解決経路の設定漏れ）。

コンストラクタの必須依存に対するセンチネルは次のとおりです。

- `ErrAIClientRequired` / `ErrResolverRequired`: `New` にクライアント / resolver が渡されなかった場合。
- `ErrVertexAIRequired`: Vertex AI 専用の resolver（`GCSResolver`）に Gemini API のクライアントを組み合わせた場合。
- `ErrGeminiAPIRequired`: Gemini API 専用の resolver（`FileAPIResolver`）に Vertex AI のクライアントを組み合わせた場合。
- `ErrReaderRequired` / `ErrHTTPClientRequired`: `NewFetchResolver` / `NewFileAPIResolver` に取得系の依存が渡されなかった場合。
- `ErrFileManagerRequired` / `ErrCacheRequired`: `NewFileAPIResolver` に `Files` / `Cache` が渡されなかった場合。

---

## 🤝 依存関係 (Dependencies)

* [shouni/go-gemini-client](https://github.com/shouni/go-gemini-client) - **Backend（Vertex AI / Google AI）を抽象化するクライアント**
* [google.golang.org/genai](https://pkg.go.dev/google.golang.org/genai) - Google Gemini 公式 SDK（`go-gemini-client` 経由の間接依存。公開 API には現れません）

---

## 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
