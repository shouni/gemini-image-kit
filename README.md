# 🎨 Gemini Image Kit

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/gemini-image-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/gemini-image-kit)](https://github.com/shouni/gemini-image-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/shouni/gemini-image-kit)](https://goreportcard.com/report/github.com/shouni/gemini-image-kit)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/gemini-image-kit.svg)](https://pkg.go.dev/github.com/shouni/gemini-image-kit)
[![Status](https://img.shields.io/badge/Status-Completed-brightgreen)](#)

## 🚀 概要 (About) - アセット運用を最適化する画像生成コア

**Gemini Image Kit** は、Google Gemini API を利用した画像生成を、Go言語でより直感的、かつ堅牢に実装するためのツールキットです。

単なる API ラッパーではなく、「**GCS/外部URLからの参照画像自動取得**」「**Gemini File API とキャッシュの一貫性管理**」「**注入可能な Downloader による取得ポリシー制御**」「**インメモリ画像圧縮**」といった、実用的なアプリケーション開発で直面する課題を解決するために設計されています。

`SingleImageRequest` による単一参照画像からの生成、`ImageFusionRequest` による複数参照画像を統合した1枚の画像生成、`EditImageRequest` による入力画像・マスク・編集プロンプトを使った画像編集をサポートします。漫画制作だけでなく、商品画像、広告素材、キャラクター差分、ゲームアセット、SNS クリエイティブなどの生成ワークフローに利用できます。

---

## ✨ 主な特徴 (Features)

* **🖼️ Unified Generator**:
  * `GenerateSingleImage`、`GenerateFusedImage`、`EditImage` により、単一参照画像の生成、複数参照画像の統合生成、画像編集を一貫して管理。
* **🧩 Image Fusion Workflow**:
  * 複数の参照画像を Gemini の入力パーツとして収集し、プロンプトと組み合わせて1枚の画像を生成。
* **🖌️ Image Edit Workflow**:
  * 入力画像、任意のマスク、編集プロンプト、対象 bbox、seed、model を `EditImageRequest` に集約。
  * Vertex AI では `gs://` の入力画像・マスクを `genai.ReferenceImage` として直接参照し、外部 URL は既存の Downloader 経由で取得。
* **🔗 Hybrid Asset Workflow**:
  * Vertex AI モード: `gs://` スキームを検知し、GCS 上のデータを転送なしで Gemini に直接参照させることで、爆速な解析とリソース節約を実現。
  * Gemini API モード: Gemini File API (`files/xxxx`) を優先利用し、キャッシュがない場合は自動的にソースから取得して再アップロードするライフサイクル管理。
* **☁️ Intelligent MIME Prediction**:
  * GCS や外部 URI からの参照時、拡張子に基づいて `MIMEType` を自動推測。SDK の `Required` 制約を透過的に解決します。
* **🛡️ Fetch Policy Injection**:
  * 外部 URL 取得は `ports.Downloader` 経由に限定。SSRF 対策や許可ドメイン制御は、アプリケーション側で安全な Downloader を注入して適用します。
* **⚡️ Optimized Image Handling**:
  * **Stream-Based Upload**: File API へのアップロードは `bufio.Reader` と `io.Pipe` を活用し、圧縮不要な場合はストリームで直接転送します。
  * **Selective Optimization**: PNG/GIF など圧縮対象の画像は JPEG に変換し、変換後の MIMEType も実データに合わせて送信します。
* **🧬 Robust Design**:
  * プロンプトとネガティブプロンプトの安全な結合、シード値の管理、アスペクト比の制御などを内蔵。

---

## 🧭 Public API

```go
// 1枚の参照画像を使って画像を生成
GenerateSingleImage(ctx, ports.SingleImageRequest)

// 複数の参照画像を統合して1枚の画像を生成
GenerateFusedImage(ctx, ports.ImageFusionRequest)

// 入力画像と任意のマスクを使って画像を編集
EditImage(ctx, ports.EditImageRequest)
```

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

    core, err := generator.NewGeminiImageCore(
       ai,
       noStorageReader{},
       httpDownloader{client: http.DefaultClient},
       newMemoryCache(),
       24*time.Hour,
       true, // PNG/GIF を JPEG に変換して送信サイズを抑える
    )
    if err != nil {
       log.Fatal(err)
    }

    g, err := generator.NewGeminiGenerator(core)
    if err != nil {
       log.Fatal(err)
    }

    resp, err := g.GenerateSingleImage(ctx, ports.SingleImageRequest{
       GenerationOptions: ports.GenerationOptions{
          Model:          "gemini-3-pro-image-preview",
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
```

> 外部 URL を受け取る場合は、`ports.Downloader` 側で許可ドメイン、IP レンジ、タイムアウト、最大サイズなどを制御してください。`http.DefaultClient` は最小例です。

### 2. 複数の参照画像を統合して生成する

```go
resp, err := g.GenerateFusedImage(ctx, ports.ImageFusionRequest{
    GenerationOptions: ports.GenerationOptions{
       Model:       "gemini-3-pro-image-preview",
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

core, err := generator.NewGeminiImageCore(
    ai,
    noStorageReader{},
    httpDownloader{client: http.DefaultClient},
    newMemoryCache(),
    24*time.Hour,
    false,
)
if err != nil {
    log.Fatal(err)
}

g, err := generator.NewGeminiGenerator(core)
if err != nil {
    log.Fatal(err)
}

resp, err := g.GenerateSingleImage(ctx, ports.SingleImageRequest{
    GenerationOptions: ports.GenerationOptions{
       Model:       "gemini-3-pro-image-preview",
       Prompt:      "この商品画像を、SNS 広告向けの高級感ある構図にしてください。",
       AspectRatio: "4:5",
       ImageSize:   "1K",
    },
    Image: ports.ImageURI{
       ReferenceURL: "gs://your-bucket/products/source.png",
    },
})
```

### 4. Vertex AI で入力画像とマスクを使って編集する

`EditImage` は Vertex AI の画像編集 API を利用します。Google AI backend では `go-gemini-client` 側から `ErrUnsupportedBackend` が返ります。

```go
seed := int64(123)

resp, err := g.EditImage(ctx, ports.EditImageRequest{
    Model:      "imagen-3.0-capability-001",
    EditPrompt: "対象領域のバッグを黒いレザーバッグに差し替えてください。",
    Image: ports.ImageURI{
       ReferenceURL: "gs://your-bucket/edit/source.png",
    },
    Mask: ports.ImageURI{
       ReferenceURL: "gs://your-bucket/edit/mask.png",
    },
    TargetBBox: &ports.BoundingBox{
       X:      120,
       Y:      80,
       Width:  320,
       Height: 240,
    },
    Seed: &seed,
})
if err != nil {
    log.Fatal(err)
}

if err := os.WriteFile("edited.png", resp.Data, 0644); err != nil {
    log.Fatal(err)
}
```

`TargetBBox` は SDK の編集 config に専用フィールドがないため、編集プロンプトへ追記してモデルに渡します。マスクを指定した場合は `MASK_MODE_USER_PROVIDED` の `MaskReferenceImage` として送信します。

---

## 📂 プロジェクト構造 (Project Structure)

```text
gemini-image-kit/
├── generator/           # 画像生成のコアロジック
│   ├── core.go          # GeminiImageCore（File API のライフサイクル管理）
│   ├── core_helper.go   # 画像フェッチ・パース処理
│   ├── gemini.go        # GeminiGenerator（高レベルジェネレーター）
│   └── gemini_helper.go # パーツ収集、プロンプト構築ロジック
├── ports/               # 外部インターフェースおよび入出力モデル定義
│   ├── interfaces.go    # ImageExecutor / ImageCacher 等の抽象化定義
│   ├── image.go         # リクエスト/レスポンス、ImageURI 等の型定義
│   └── image_helpers.go # ドメインモデルに関連するヘルパー関数
└── imgutil/             # 画像処理ユーティリティ
    ├── mime.go          # MIMEタイプ判定ロジック
    └── compressor.go    # 送信前画像圧縮（JPEG最適化等）
```

---

## 🤝 依存関係 (Dependencies)

* [google.golang.org/genai](https://pkg.go.dev/google.golang.org/genai) - Google Gemini 公式 SDK
* [shouni/go-gemini-client](https://github.com/shouni/go-gemini-client) - **Backend（Vertex AI / Google AI）を抽象化するクライアント**

---

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
