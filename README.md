# 🎨 Gemini Image Kit

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/gemini-image-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/gemini-image-kit)](https://github.com/shouni/gemini-image-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)


# 🎨 Gemini Image Kit

## 🚀 概要 (About) - Gemini File APIをスマートに管理。アセット運用を最適化する画像生成コア

**Gemini Image Kit** は、Google Gemini API を利用した画像生成を、Go言語でより直感的、かつ堅牢に実装するためのツールキットです。

単なる API ラッパーではなく、「**GCS/外部URLからの参照画像自動取得**」「**Gemini File API とキャッシュの一貫性管理**」「**SSRFプロテクション**」「**インメモリ画像圧縮**」といった、実用的なアプリケーション開発で直面する課題を解決するために設計されています。

---

## ✨ 主な特徴 (Features)

* **🖼️ Unified Generator**: プロンプト構築から生成までを一貫して管理。
* **🔗 Intelligent Asset Fallback**:
* Gemini File API (`files/xxxx`) を優先利用し、キャッシュがない場合は自動的にソースから取得して再アップロードするライフサイクル管理。


* **☁️ Cloud Storage Native**: `gs://` スキームを標準サポート。キャラクターデザインなどのアセットを GCS から直接参照可能。
* **🛡️ SSRF Protected**: 外部 URL 取得時、名前解決後の IP レベルで内部ネットワークへのアクセスを遮断するバリデーション。
* **⚡️ Built-in Image Optimization**:
* 送信前に画像をインメモリで最適化（JPEG 圧縮）し、ペイロードサイズを抑えて高速な生成を実現。


* **🧬 Robust Design**:
* インターフェース分離により、モックを利用したテストが容易。
* プロンプトとネガティブプロンプトの安全な結合ロジックを内蔵。


---

## 🛠️ クイックスタート (Usage)

### 1. ジェネレーターの初期化

```go
import (
    "time"
    "github.com/shouni/gemini-image-kit/pkg/generator"
)

// 1. 基盤となる Core の準備
core, err := generator.NewGeminiImageCore(aiClient, reader, httpClient, cache, 24*time.Hour)
if err != nil {
    log.Fatal(err)
}

// 2. ジェネレーターの生成
gen, err := generator.NewGeminiGenerator("imagen-3.0-generate-001", core)

```

### 2. 画像の生成

```go
req := domain.ImageGenerationRequest{
    Prompt:       "サイバーパンクな街に立つキャラクター",
    ReferenceURL: "gs://my-bucket/char_design.png", // GCSから自動取得
    AspectRatio:  "16:9",
}

resp, err := gen.GenerateMangaPanel(ctx, req)

```

---

## 📂 プロジェクト構造 (Layout)

```text
pkg/
├── domain/            # 共通ドメインモデル
│   └── image.go       # リクエスト/レスポンスの型定義
├── generator/         # 画像生成のコアロジック
│   ├── interfaces.go  # ImageExecutor / ImageCacher 等の抽象化定義
│   ├── gemini.go      # 高レベルジェネレーター（フォールバック制御）
│   ├── core.go        # GeminiImageCore（File API のライフサイクル管理）
│   ├── core_helper.go # 画像フェッチ・パース処理
│   └── types.go       # パッケージ内部用定数・型定義
├── imgutil/           # 画像処理ユーティリティ
│   └── compressor.go  # 送信前画像圧縮（JPEG最適化）
└── utils/             # 共通ヘルパー
    └── util.go        # プロンプト構築・シード値変換・SSRFバリデーション

```

---

## 🤝 依存関係 (Dependencies)

* [google.golang.org/genai](https://pkg.go.dev/google.golang.org/genai) - Google Gemini 公式 SDK
* [shouni/go-gemini-client](https://github.com/shouni/go-gemini-client) - Gemini API 通信の抽象化
* [shouni/go-remote-io](https://github.com/shouni/go-remote-io) - GCS/HTTP マルチストレージ Reader
* [shouni/go-utils](https://github.com/shouni/go-utils) - 汎用ユーティリティ

---

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。

---
