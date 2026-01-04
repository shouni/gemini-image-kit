# 🎨 Gemini Image Kit

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/gemini-image-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/gemini-image-kit)](https://github.com/shouni/gemini-image-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)


## 🚀 概要 (About) - 画像生成の「面倒」を解決する、Gemini 抽象化ライブラリ

**Gemini Image Kit** は、Google Gemini API を利用した画像生成を、Go言語でより直感的、かつ堅牢に実装するためのツールキットなのだ。

単なる API ラッパーではなく、「**参照画像の自動ダウンロード・キャッシュ**」「**SSRFプロテクション**」「**マルチモーダルなパーツ組み立て**」「**SDK互換のシード値管理**」といった、実用的なアプリケーション開発で直面する課題を解決するために設計されているのだ。

---

## ✨ 主な特徴 (Features)

* **🖼️ Multi-Modal Orchestration**: テキストと複数の参照画像（URL）を組み合わせた高度なプロンプト構築を数行で実現。単一パネル生成に加え、複数画像を参照する一括ページ生成にも対応。
* **🛡️ SSRF Protected**: ユーザー指定のURLから画像を生成する際、内部ネットワークへの攻撃（SSRF）を防ぐため、名前解決レベルでのIP制限バリデーションを標準装備。
* **⚡️ Built-in Image Caching**: 同一URLの参照画像を何度もダウンロードしないためのキャッシュ機構（`ImageCacher`）を搭載。
* **🧬 Seed Consistency**: Gemini SDK 特有の `*int32` Seed値を扱いやすくカプセル化し、生成結果の再現性をサポート。
* **ログ・デバッグ支援**: 生成プロセスの詳細（パーツ構成、ブロック理由等）を `slog` で可視化。

---

## 🛡️ セキュリティ (Security)

本ライブラリは、サーバーサイドで外部URLを取得する際の **SSRF (Server-Side Request Forgery)** 攻撃を防ぐため、以下の安全策を講じています。

* **IP制限**: `localhost`、プライベートIP、リンクローカルアドレスといった、内部ネットワークへのリクエストを名前解決後のIPレベルでブロック。
* **プロトコル制限**: `http` および `https` 以外の不許可スキームを拒否。

---

## 📂 プロジェクト構造 (Layout)

```text
pkg/
├── domain/            # 共通ドメインモデル（Request/Response 等）
└── adapters/          # 具体的な実装（アダプター層）
    ├── core.go        # 画像DL、キャッシュ、パース、SSRF対策の基盤 (GeminiImageCore)
    ├── generator.go   # 単一パネル・画像生成 (GeminiImageGenerator)
    ├── page_gen.go    # 複数画像を含むページ一括生成 (GeminiMangaPageGenerator)
    └── util.go        # シード値変換等のユーティリティ

```

---

## 🛠️ クイックスタート (Usage)

### 1. コアロジックとジェネレーターの初期化

```go
import (
    "github.com/shouni/gemini-image-kit/pkg/adapters"
    "github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
)

// 1. 画像処理・キャッシュ・セキュリティを担当する Core の準備
core := adapters.NewGeminiImageCore(httpClient, cache, 1*time.Hour)

// 2. 単一パネル生成用ジェネレーターの生成
generator := adapters.NewGeminiImageGenerator(
    core,
    apiClient,
    "imagen-3.0-generate-001",
)

```

### 2. 単一パネルの生成

```go
req := domain.ImageGenerationRequest{
    Prompt:       "ずんだもんが森で餅を食べている",
    AspectRatio:  "16:9",
    ReferenceURL: "https://example.com/character_sheet.png",
    Seed:         ptrInt64(12345),
}

resp, err := generator.GenerateMangaPanel(ctx, req)
// resp.Data に画像バイナリが含まれるのだ！

```

### 3. 複数画像を参照した一括ページ生成

```go
pageGen := adapters.NewGeminiMangaPageGenerator(core, apiClient, "imagen-3.0")

req := domain.ImagePageRequest{
    Prompt: "二人のキャラクターが対峙している緊迫したシーン",
    ReferenceURLs: []string{
        "https://example.com/hero.png",
        "https://example.com/villain.png",
    },
    AspectRatio: "3:4",
}

resp, err := pageGen.GenerateMangaPage(ctx, req)

```

---

## 🤝 依存関係 (Dependencies)

* [google.golang.org/genai](https://pkg.go.dev/google.golang.org/genai) - Google Gemini API 公式クライアント
* [shouni/go-ai-client](https://github.com/shouni/go-ai-client) - AI通信の抽象化
* [shouni/go-http-kit](https://github.com/shouni/go-http-kit) - 堅牢な HTTP クライアント

---

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
