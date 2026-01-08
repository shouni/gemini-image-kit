# 🎨 Gemini Image Kit

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/gemini-image-kit)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/gemini-image-kit)](https://github.com/shouni/gemini-image-kit/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)


## 🚀 概要 (About) - 画像生成の「面倒」を解決する、Gemini 抽象化ライブラリ

**Gemini Image Kit** は、Google Gemini API を利用した画像生成を、Go言語でより直感的、かつ堅牢に実装するためのツールキットなのだ。

単なる API ラッパーではなく、「**参照画像の自動ダウンロード・キャッシュ**」「**SSRFプロテクション**」「**インメモリ画像圧縮**」「**SDK互換のシード値管理**」といった、実用的なアプリケーション開発で直面する課題を解決するために設計されているのだ。

---

## ✨ 主な特徴 (Features)

* **🖼️ Unified Generator**: 統合された `GeminiGenerator` により、単独パネル生成と複数参照ページ生成の両方を一つのインスタンスで提供。
* **🛡️ SSRF Protected**: 外部URLから画像を読み込む際、名前解決後のIPレベルで内部ネットワークへのアクセスを遮断するバリデーションを標準装備。
* **⚡️ Built-in Image Caching & Compression**:
* 同一URLの参照画像を再利用する `ImageCacher` インターフェースにより通信量を削減。
* 送信前に画像を最適化（JPEG圧縮）する機能を備え、ペイロードサイズを抑えて高速な生成を実現。


* **🧬 Seed Consistency**: `*int64` (Domain) と `*int32` (Gemini SDK) の型変換をカプセル化し、一貫したシード値管理を実現。
* **🪵 slog Integration**: 生成プロセス（パーツ構成、ブロック理由等）を構造化ログで可視化。

---

## 🛡️ セキュリティ (Security)

本ライブラリは、サーバーサイドで外部URLを取得する際の **SSRF (Server-Side Request Forgery)** 攻撃を防ぐため、以下の安全策を講じています。

* **IP制限**: `isSafeURL` 関数により、`localhost`、プライベートIP、リンクローカルアドレスといった内部ネットワークへのリクエストをブロック。
* **DNSリバインディング対策**: 名前解決の結果得られたすべてのIPを検証。
* **プロトコル制限**: `http` および `https` 以外の不許可スキームを拒否。

---

## 📂 プロジェクト構造 (Layout)

```text
pkg/
├── domain/            # 共通ドメインモデル（Request/Response 等）
├── generator/         # 統合パッケージ
│   ├── interfaces.go  # 生成インターフェース定義
│   ├── gemini.go      # GeminiGenerator 実装
│   ├── core.go        # 画像DL、キャッシュ、パース基盤
│   └── util.go        # SSRF対策・型変換ユーティリティ
└── imgutil/           # 画像処理ユーティリティ
    └── compressor.go  # JPEG圧縮・フォーマット変換

```

---

## 🛠️ クイックスタート (Usage)

### 1. ジェネレーターの初期化

`NewGeminiGenerator` は依存関係の `nil` チェックを行うため、安全に初期化できるのだ。

```go
import (
    "github.com/shouni/gemini-image-kit/pkg/generator"
    "github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
)

// 1. 基盤となる Core の準備
// httpClient, cache, expiration を注入するのだ
core, err := generator.NewGeminiImageCore(httpClient, cache, 1*time.Hour)
if err != nil {
    log.Fatal(err)
}

// 2. 統合ジェネレーターの生成
gen, err := generator.NewGeminiGenerator(core, apiClient, "imagen-3.0-generate-001")
if err != nil {
    log.Fatal(err)
}

```

### 2. 画像の生成（パネル or ページ）

一つのインスタンスで柔軟なインターフェースを使い分けられるのだ。

```go
// --- 単一パネルの生成 ---
panelReq := domain.ImageGenerationRequest{
    Prompt:       "青い空を飛ぶ白い鳥",
    AspectRatio:  "16:9",
    ReferenceURL: "https://example.com/ref.png",
}
panelResp, err := gen.GenerateMangaPanel(ctx, panelReq)

// --- 複数画像を参照したページ一括生成 ---
pageReq := domain.ImagePageRequest{
    Prompt: "二人のキャラクターが対話しているシーン",
    ReferenceURLs: []string{
        "https://example.com/char_a.png",
        "https://example.com/char_b.png",
    },
    AspectRatio: "3:4",
}
pageResp, err := gen.GenerateMangaPage(ctx, pageReq)

```

---

## 🤝 依存関係 (Dependencies)

* [google.golang.org/genai](https://pkg.go.dev/google.golang.org/genai) - Google Gemini API 公式クライアント
* [shouni/go-ai-client](https://github.com/shouni/go-ai-client) - AI通信の抽象化
* [shouni/go-http-kit](https://github.com/shouni/go-http-kit) - 堅牢な HTTP クライアント

---

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
