# 🎨 Gemini Image Kit

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/gemini-image-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/gemini-image-kit)](https://github.com/shouni/gemini-image-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)


## 🚀 概要 (About) - 画像生成の「面倒」を解決する、Gemini 抽象化ライブラリ

**Gemini Image Kit** は、Google Gemini API を利用した画像生成を、Go言語でより直感的、かつ堅牢に実装するためのツールキットなのだ。

単なる API ラッパーではなく、「**GCS/外部URLからの参照画像自動取得**」「**SSRFプロテクション**」「**インメモリ画像圧縮**」「**SDK互換のシード値管理**」といった、実用的なアプリケーション開発で直面する課題を解決するために設計されているのだ。

---

## ✨ 主な特徴 (Features)

* **🖼️ Unified Generator**: `GenerateMangaPanel` (単独) と `GenerateMangaPage` (複数参照) を一つのインターフェースで統合管理。
* **☁️ Cloud Storage Native**: `gs://` スキームを標準サポート。キャラクターデザインシートなどのアセットを GCS から直接参照可能。
* **🛡️ SSRF Protected**: 外部 URL 取得時、名前解決後の IP レベルで内部ネットワークへのアクセスを遮断するバリデーションを標準装備。
* **⚡️ Built-in Image Caching & Compression**:
* 同一 URL の再取得を防ぐ `ImageCacher` によりコストと通信量を削減。
* 送信前に画像を最適化（JPEG 圧縮）し、ペイロードサイズを抑えて高速な生成を実現。


* **🧬 Seed Consistency**: `*int64` (Domain) と `*int32` (Gemini SDK) の変換を自動化し、一貫したシード値管理を実現。
* **🪵 slog Integration**: 構造化ログにより、プロンプトの構成やブロック理由を詳細に可視化。

---

## 🛠️ クイックスタート (Usage)

### 1. ジェネレーターの初期化

`NewGeminiImageCore` には GCS 読み込み用の `InputReader` と HTTP 取得用の `HTTPClient` を注入するのだ。

```go
import (
    "time"
    "github.com/shouni/gemini-image-kit/pkg/generator"
    "github.com/shouni/go-remote-io/pkg/remoteio"
)

// 1. 基盤となる Core の準備
// reader (GCS対応), httpClient, cache, 有効期限をセット
core, err := generator.NewGeminiImageCore(reader, httpClient, cache, 24*time.Hour)
if err != nil {
    log.Fatal(err)
}

// 2. 統合ジェネレーターの生成
gen, err := generator.NewGeminiGenerator(core, apiClient, "imagen-3.0-generate-001")
if err != nil {
    log.Fatal(err)
}

```

### 2. 画像の生成（GCS URL の活用）

`ReferenceURLs` に `gs://` スキームを含めることで、クラウド上のアセットをシームレスに合成のヒントとして利用できるのだ。

```go
// --- 複数画像を参照したページ生成 ---
req := domain.ImagePageRequest{
    Prompt: "このキャラクターがサイバーパンクな街並みに立っている様子",
    ReferenceURLs: []string{
        "gs://my-bucket/assets/char_design.png", // GCSから直接読み込み
        "https://example.com/background_style.jpg",
    },
    AspectRatio: "16:9",
}

resp, err := gen.GenerateMangaPage(ctx, req)
if err != nil {
    log.Printf("生成エラー: %v", err)
}

```

---

## 🛡️ セキュリティ (Security)

本ライブラリは、SSRF (Server-Side Request Forgery) 攻撃を防ぐため、以下の安全策を講じています。

* **IP 制限**: `localhost`、プライベート IP、リンクローカルアドレスへのリクエストを強制ブロック。
* **DNS 対策**: 名前解決されたすべての IP アドレスを検証。
* **スキーム制限**: `http`, `https`, `gs` 以外の不許可プロトコルを拒否。

---

## 📂 プロジェクト構造 (Layout)

```text
pkg/
├── domain/            # 共通ドメインモデル
├── generator/         # コアロジック
│   ├── interfaces.go  # インターフェース定義
│   ├── gemini.go      # ジェネレーター実装
│   ├── core.go        # 画像取得・キャッシュ・圧縮・パース
│   └── util.go        # SSRF対策・型変換
└── imgutil/           # 画像処理ユーティリティ

```

---

## 🤝 依存関係 (Dependencies)

* [google.golang.org/genai](https://pkg.go.dev/google.golang.org/genai) - Google Gemini 公式 SDK
* [shouni/go-ai-client](https://github.com/shouni/go-ai-client) - AI 通信の抽象化
* [shouni/go-remote-io](https://github.com/shouni/go-remote-io) - マルチストレージ Reader

---

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
