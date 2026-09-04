# 🎨 Gemini Image Kit

[![CI](https://github.com/shouni/gemini-image-kit/actions/workflows/ci.yml/badge.svg)](https://github.com/shouni/gemini-image-kit/actions/workflows/ci.yml)
[![Status](https://img.shields.io/badge/Status-Active-brightgreen)](#)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://go.dev/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/gemini-image-kit)](https://go.dev/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/gemini-image-kit)](https://github.com/shouni/gemini-image-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/gemini-image-kit.svg)](https://pkg.go.dev/github.com/shouni/gemini-image-kit)

## 🚀 概要 (About) - 窓口は Generate 1 つ。参照画像の送り方はアプリが選ぶ

**Gemini Image Kit** は、Gemini API / Vertex AI 上の画像モデルによる画像生成を引き受ける Go ライブラリです。
生成の入口は `ports.ImageGenerator` の `Generate(ctx, ImageRequest)` ひとつで、参照画像が 0 枚でも複数枚でも
同じメソッドです（`Images` の枚数が「テキストのみ / 単一参照 / 融合」の解釈を決めます）。既存画像の編集も、
編集対象を参照に、編集指示をプロンプトにして `Generate` を呼ぶだけです。

**呼び出しガード（発射間隔・上限時間・重複排除）と、生成物の保存は持ちません。** 前者はワークフロー層、
後者は呼び出し側の担当です。外部 URL の取得も `ports.Downloader` の注入に限定しており、SSRF 対策と
ドメイン許可リストはアプリケーションが決めます。

### `genai-kit` の `imagegen` との使い分け

参照画像の**取得・サイズ上限・再圧縮・アップロードのキャッシュが要る**経路を担当するのがこちらです。
Vertex AI だけを使い、参照画像が `gs://` で足りるなら、モデル側が `gs://` を解決するので転送そのものが
発生しません — その構成は [genai-kit](https://github.com/shouni/genai-kit) の `imagegen` を使ってください。
本キットは Gemini API（API キー方式）も扱い、File API へ上げて使い回す経路を持ちます。

---

## ✨ 提供機能 (Features)

* **参照画像の送り方は注入で決まる** — `gs://` を転送せず直接参照する `GCSResolver`、取得してインライン送信する
  `FetchResolver`、File API へ上げてキャッシュする `FileAPIResolver` を `ResolverChain` で並べます。
  **`generator.New` の第 2 引数 `resolver` は必須で、既定はありません** — 運用上の判断をキットが黙って
  選ばないためです。依存は選んだ resolver だけが要求するので、`gs://` しか使わない構成では取得・キャッシュの
  実装を渡す必要がありません。
* **バックエンド制約は構築時に弾く** — Vertex AI 専用の `GCSResolver` を Gemini API のクライアントと、
  Gemini API 専用の `FileAPIResolver` を Vertex AI のクライアントと組み合わせると `New` がエラーを返します。
  **後者は放っておくと生成が「成功してしまう」** ぶん気付きにくく、毎回インライン経路へ落ちて `gs://` を
  二度ダウンロードし続けます。バックエンドを申告しないクライアント（テスト用フェイクなど）は素通しします。
* **MIME は推測できなければ付けない** — URI 参照では拡張子から、インライン送信では実データから判定します。
  既定値を当てると PNG を JPEG と申告する誤った型宣言になりうるためで、分からないことは分からないまま
  サーバー側の判定に委ねます。
* **シードは送る前に決める** — API はレスポンスにシードを返さないため、未指定のリクエストには生成側で
  採番してから送信します。`ImageResponse.UsedSeed` は常に実際に送った値で、記録すれば再現できます
  （自前で管理するなら `WithoutAutoSeed()`）。
* **転送は必要なぶんだけ** — `Compress` を有効にすると PNG/GIF だけを JPEG へ再圧縮します。対象外の形式は
  デコードせずストリームのまま流します。同一ソースへの同時アップロードは singleflight で 1 回にまとまります。
* **応答は記録に足りる** — `ImageResponse` は画像バイト列に加えて `Model` / `Prompt` / `Usage` を返すので、
  コスト集計や生成条件の記録にリクエストを持ち回る必要がありません。
* **公開 API に `google.golang.org/genai` の型は現れません** — 生成 SDK の型は
  [go-gemini-client](https://github.com/shouni/go-gemini-client) の内側に閉じています。

---

## 📦 パッケージ構成 (Package Structure)

```text
gemini-image-kit/
├── ports/       # 公開インターフェースと入出力モデル（ImageGenerator / ReferenceResolver /
│                #   ImageCacher / ContentReader / Downloader、ImageRequest / ImageResponse など）
│                #   保存先の拡張子を引く ExtensionByMIMEType もここ
└── generator/   # 画像生成の実装（Generator）と resolver 群
                 #   （GCSResolver / FetchResolver / FileAPIResolver / ResolverChain）
```

`generator` は `ports` のインターフェースに対して実装されているので、利用側は `ports` の型だけを参照して
差し替えやモックができます。利用側が依存するポートは `ports.ImageGenerator`（1 メソッド）です。

---

## 🚦 使い方 (Usage)

Gemini API バックエンドで、外部 URL の参照画像 1 枚から生成するまでです。

```go
ai, err := gemini.NewClient(ctx, gemini.Config{APIKey: os.Getenv("GEMINI_API_KEY")})
if err != nil {
    log.Fatal(err)
}

// File API へ上げて使い回し、扱えなければ取得してインラインへ落とす
upload, err := generator.NewFileAPIResolver(generator.FileAPIResolverConfig{
    Files:      ai,
    Reader:     noStorageReader{},                      // gs:// を使わないなら開かない実装で足ります
    Downloader: httpDownloader{client: http.DefaultClient},
    Cache:      newMemoryCache(),
    CacheTTL:   24 * time.Hour,
    Compress:   true,
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
        Model:           "gemini-3-pro-image",
        Prompt:          "参照画像の人物を、白背景の商品広告風ポートレートにしてください。",
        NegativePrompt:  "low quality, blurry, distorted hands",
        GenerateOptions: gemini.GenerateOptions{AspectRatio: "1:1", ImageSize: "1K"},
    },
    Images: []ports.ImageURI{{ReferenceURL: "https://example.com/reference.png"}},
})
if err != nil {
    log.Fatal(err)
}

// 保存は呼び出し側の責務。拡張子は ports.ExtensionByMIMEType で引けます。
os.WriteFile("output"+ports.ExtensionByMIMEType(resp.MimeType), resp.Data, 0o644)
```

> `http.DefaultClient` は最小例です。**参照画像の URL は呼び出し側の入力がそのまま渡る**ので、許可ドメイン・
> IP レンジ・タイムアウト・最大サイズは `ports.Downloader` の実装側で制御してください。

分岐や応用（Vertex AI + `gs://`、複数参照の融合、既存画像の編集）は
[pkg.go.dev](https://pkg.go.dev/github.com/shouni/gemini-image-kit) の各型のドキュメントにあります。

<details>
<summary>上の例で使っている補助実装（最小のプレースホルダ）</summary>

`Reader` / `Downloader` / `Cache` は注入する前提なので、動かすための最小実装を載せます。実運用では、
上の注意を踏まえた HTTP クライアント、GCS 読み取り、TTL 付きキャッシュ（`ttlcache` など）に置き換えてください。
**`ports.ImageCacher` は同時アクセス安全である必要があります**（参照の解決は並行に走ります）。

```go
type httpDownloader struct{ client *http.Client }

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

func newMemoryCache() *memoryCache { return &memoryCache{data: make(map[string]any)} }

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

---

## 🎯 参照画像の解決方法

`ImageURI` 1 件をどう送るかは、注入した `ports.ReferenceResolver` が決めます。典型的な並べ方は 3 つです。

```go
// Vertex AI: gs:// は転送せず直接参照、それ以外は取得してインライン
generator.NewResolverChain(generator.NewGCSResolver(), fetchResolver)

// Gemini API: File API へ上げて使い回し、失敗したら取得してインライン
generator.NewResolverChain(fileAPIResolver, fetchResolver)

// 参照が gs:// だけの構成（取得もキャッシュも要りません）
generator.NewGCSResolver()
```

`ResolverChain` が次へ進むのは `ports.ErrResolverNotApplicable`（管轄外）を返した resolver だけで、
取得失敗のような実エラーはその場で返します — 次へ流すと、ネットワーク障害が「参照を解決できません」に
すり替わって原因が消えるためです。誰も扱えなければ `ErrUnresolvedReference`（経路の設定漏れ）になります。

**`FileAPIResolver` はアップロードに失敗すると警告ログを出して辞退し、次の resolver に委ねます。**
アップロードは送信量を減らすための最適化であって正しさの要件ではないためで、失敗しても生成は続きます
（ただし失敗の原因が呼び出し側のキャンセルであれば、辞退せずそのまま返します）。
`FetchResolver` を最後段に置かないと、この経路が受け皿を失います。

参照の解決は複数画像に対して**並行**に走ります。注入する resolver とそのキャッシュは同時アクセス安全で
なければなりません。並び順は入力順のまま保たれます（モデルの解釈に影響するためです）。

---

## 🚥 呼び出しガードの掛け方

発射間隔・1 回あたりの上限時間・同一内容の重複排除は、このキットではなく**ワークフロー層**に置いてください。
Gemini のクォータはプロジェクト単位で操作の種類ごとではないため、画像生成だけを絞ってもテキスト生成が同じ
クォータを消費してしまいます。`go-gemini-client/callguard` の `Guard` を 1 つ作り、`ports.ImageGenerator` を
デコレートして、テキスト生成と同じ `Guard` を共有させるのが定型です。
**相乗りした呼び出し元は同じ `*ImageResponse` を共有する**ので、返す前に複製してください。

**参照画像のアップロードはこのガードを通さないでください。** File API へのアップロードは生成呼び出しでは
なくクォータを消費しないため、生成の発射枠を消費させると参照画像 1 枚ごとに発射間隔ぶん待つことになります
（5 枚・30 秒間隔なら、生成が始まる前に 2 分半）。アップロードの上限時間は
`FileAPIResolverConfig.UploadTimeout` で別に指定します。

---

## 🤝 依存関係 (Dependencies)

* [shouni/go-gemini-client](https://github.com/shouni/go-gemini-client) - Gemini API / Vertex AI の
  バックエンドを抽象化するクライアント
* [google.golang.org/genai](https://pkg.go.dev/google.golang.org/genai) - Google Gemini 公式 SDK
  （`go-gemini-client` 経由の間接依存。公開 API には現れません）

---

## 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
