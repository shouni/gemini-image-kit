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
* **🔗 Intelligent Asset Fallback**:
    * Gemini File API (`files/xxxx`) を優先利用し、キャッシュがない場合は自動的に `ReferenceURL` からのインライン送信にフォールバック。
* **☁️ Cloud Storage Native**: `gs://` スキームを標準サポート。キャラクターデザインシートなどのアセットを GCS から直接参照可能。
* **🛡️ SSRF Protected**: 外部 URL 取得時、名前解決後の IP レベルで内部ネットワークへのアクセスを遮断するバリデーションを標準装備。
* **⚡️ Built-in Image Optimization**:
    * 送信前に画像を最適化（JPEG 圧縮）し、ペイロードサイズを抑えて高速な生成を実現。
    * プロンプトとネガティブプロンプトの安全な結合ロジックを内蔵。
* **🧬 Robust Error Handling**:
    * `FinishReason` の詳細な検証により、セーフティフィルターによるブロックなどの原因を明確に特定。
    * インターフェース分離（`ImageExecutor`）による高いテスト容易性。

---

## 🛠️ クイックスタート (Usage)

### 1. ジェネレーターの初期化

`NewGeminiImageCore` で基盤を作り、それを `NewGeminiGenerator` に注入するのだ。

```go
import (
    "time"
    "github.com/shouni/gemini-image-kit/pkg/generator"
)

// 1. 基盤となる Core (ImageExecutor) の準備
// aiClient, reader, httpClient, cache, 有効期限をセット
core, err := generator.NewGeminiImageCore(aiClient, reader, httpClient, cache, 24*time.Hour)
if err != nil {
    log.Fatal(err)
}

// 2. 統合ジェネレーターの生成 (Coreインターフェースを注入)
gen, err := generator.NewGeminiGenerator("imagen-3.0-generate-001", core)
if err != nil {
    log.Fatal(err)
}

```

### 2. 画像の生成（File API と URL の自動使い分け）

`FileAPIURIs` に値があればそれを優先し、空の場合は `ReferenceURLs` から画像を取得して送信するのだ。

```go
req := domain.ImagePageRequest{
    Prompt: "サイバーパンクな街に立つキャラクター",
    NegativePrompt: "low quality, blurry",
    FileAPIURIs: []string{"https://generativelanguage.googleapis.com/v1beta/files/asset-123"},
    ReferenceURLs: []string{"gs://my-bucket/char_design.png"}, // FileAPIURIsが空ならこちらを使用
    AspectRatio: "16:9",
}

resp, err := gen.GenerateMangaPage(ctx, req)

```

---

## 📂 プロジェクト構造 (Layout)

```text
pkg/
├── domain/            # 共通ドメインモデル（リクエスト・レスポンス定義）
├── generator/         # 画像生成コアロジック
│   ├── interfaces.go  # ImageExecutor 等の抽象化インターフェース
│   ├── gemini.go      # 高レベルジェネレーター（プロンプト構築・フォールバック）
│   ├── core.go        # File API 操作（Upload/Delete）とライフサイクル管理
│   ├── core_helper.go # 画像取得・パース・バリデーション
│   ├── types.go       # パッケージ内部用型定義
│   └── util.go        # SSRF対策・シード値変換・URL検証
└── imgutil/           # 画像処理（圧縮・リサイズ）ユーティリティ

```

---

## 🤝 依存関係 (Dependencies)

* [google.golang.org/genai](https://pkg.go.dev/google.golang.org/genai) - Google Gemini 公式 SDK
* [shouni/go-gemini-client](https://github.com/shouni/go-gemini-client) - Gemini API 通信の抽象化
* [shouni/go-remote-io](https://github.com/shouni/go-remote-io) - GCS/HTTP マルチストレージ Reader

---

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
