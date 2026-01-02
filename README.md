# 🎨 Gemini Image Kit

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/gemini-image-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/gemini-image-kit)](https://github.com/shouni/gemini-image-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)


## 🚀 概要 (About) - 画像生成の「面倒」を解決する、Gemini 抽象化ライブラリ

**Gemini Image Kit** は、Google Gemini API を利用した画像生成を、Go言語でより直感的、かつ堅牢に実装するためのツールキットなのだ。

単なる API ラッパーではなく、「**参照画像の自動ダウンロード・キャッシュ**」「**マルチモーダルなパーツ組み立て**」「**SDK互換のシード値管理**」といった、実践的なアプリケーション開発で必ず直面する「共通の課題」を解決するために設計されているのだ。

---

## ✨ 主な特徴 (Features)

* **🖼️ Multi-Modal Orchestration**: テキストと複数の参照画像（URL）を組み合わせた高度なプロンプト構築を数行で実現。
* **⚡️ Built-in Image Caching**: 同一URLの参照画像を何度もダウンロードしないためのキャッシュ機構（`ImageCacher`）を標準搭載。
* **🛠️ Domain-Driven Design**: `domain` パッケージに型を定義し、ビジネスロジックが Gemini SDK の内部仕様に依存しすぎないクリーンな設計。
* **🧬 Seed Consistency**: Gemini SDK 特有の `*int32` Seed値を扱いやすくカプセル化し、生成結果の再現性をサポート。
* **ログ・デバッグ支援**: 生成プロセスの詳細（パーツ構成、ブロック理由等）を `slog` で可視化。

---

## 📂 プロジェクト構造 (Layout)

```text
pkg/
├── domain/            # 共通ドメインモデル（Request/Response, Character定義など）
│   └── manga.go       # 漫画・画像生成に関するデータ構造
└── adapters/          # 具体的な実装（アダプター層）
    ├── core.go        # 画像DL、キャッシュ、パースの共通基盤 (GeminiImageCore)
    ├── image.go       # 単体パネル・画像生成 (GeminiImageAdapter)
    └── manga.go       # 複数画像を含むページ一括生成 (GeminiMangaPageAdapter)

```

---

## 🛠️ クイックスタート (Usage)

### 1. Adapter の初期化

```go
import (
    "ap-manga-go/pkg/adapters"
    "github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
)

// コアロジックの準備
core := adapters.NewGeminiImageCore(httpClient, cache, 1*time.Hour)

// アダプターの生成
adapter := adapters.NewGeminiImageAdapter(
    core,
    apiClient,
    "imagen-3.0-generate-001",
    "anime style, high quality, manga illustration",
)

```

### 2. 画像の生成

```go
req := domain.ImageGenerationRequest{
    Prompt:       "ずんだもんが森で餅を食べている",
    AspectRatio:  "16:9",
    ReferenceURL: "https://example.com/zundamon.png",
}

resp, err := adapter.GenerateMangaPanel(ctx, req)
if err != nil {
    log.Fatal(err)
}

// resp.Data に画像バイナリが含まれるのだ！

```

---

## 🤝 依存関係 (Dependencies)

* [google.golang.org/genai](https://pkg.go.dev/google.golang.org/genai) - Google Gemini API 公式クライアント
* [shouni/go-ai-client](https://www.google.com/search?q=https://github.com/shouni/go-ai-client) - AI通信の抽象化
* [shouni/go-http-kit](https://www.google.com/search?q=https://github.com/shouni/go-http-kit) - 堅牢な HTTP クライアント

---

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。


