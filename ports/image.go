// Package ports は、gemini-image-kit の各コンポーネントが依存する
// インターフェース（ポート）と、画像生成・編集に関する共通データ型を定義します。
package ports

import (
	"github.com/shouni/go-gemini-client/gemini"
)

// ImageURI は参照画像の所在を表します。
//
// どちらのフィールドをどう使うかは、注入した ReferenceResolver が決めます。
type ImageURI struct {
	// ReferenceURL は元の参照先です（gs:// または http(s)://）。
	ReferenceURL string
	// FileAPIURI は、呼び出し側が既に Gemini File API へ上げてある場合の URI です
	// (https://...)。generator.FileAPIResolver を経路に置いている場合のみ使われ、
	// 指定するとアップロードが省かれます。それ以外の経路では無視されます。
	FileAPIURI string
}

// IsEmpty は参照先が設定されていないかを返します。
//
// 空の要素はエラーではなく、送信対象から黙って外れます。「このキャラクターには
// 参照画像が無い」を、呼び出し側が要素の欠落として表現できるようにするためです。
func (uri ImageURI) IsEmpty() bool {
	return uri.ReferenceURL == "" && uri.FileAPIURI == ""
}

// GenerationOptions は画像生成時の共通設定パラメータを保持します。
//
// gemini.GenerateOptions を埋め込んでいるため、SystemPrompt / AspectRatio /
// ImageSize / Seed / Temperature などの生成パラメータはフィールド昇格でそのまま
// 設定できます。フィールドを写し取ると go-gemini-client 側の追加のたびに 2 ファイルの
// 同期が必要になるため、埋め込みにしています。
//
// SafetySettings と PersonGeneration は、未指定の場合のみバックエンドに応じた
// 既定値（安全フィルタ無効・人物生成許可）が補われます。呼び出し側が明示した
// 値は上書きされません。
type GenerationOptions struct {
	Model          string
	Prompt         string
	NegativePrompt string

	gemini.GenerateOptions
}

// ImageRequest は画像生成 1 回分の要求です。
//
// Images が 1 枚なら参照付きの単発生成、複数なら参照画像を統合した融合生成、
// 空ならテキストのみの生成になります。枚数が解釈を決めるため、単発と融合で
// 型を分けません。
type ImageRequest struct {
	GenerationOptions
	// Images は参照画像です。並び順はモデルの解釈に影響するため保持されます。
	Images []ImageURI
}

// ImageResponse は生成された画像データとそのメタデータです。
type ImageResponse struct {
	Data     []byte
	MimeType string
	// UsedSeed はリクエストで指定した（または自動採番された）シードです。
	// API はレスポンスにシードを返さないため、これは送信値の記録です。
	UsedSeed int64
	// Model は生成に使ったモデル名です。コスト集計やメタデータ保存のために、
	// 呼び出し側がリクエストから別途持ち回らずに済むよう応答へ含めます。
	Model string
	// Prompt は実際に送信した最終プロンプト（ネガティブプロンプト結合済み）です。
	Prompt string
	// Usage はトークン使用量です。モデルが返さない場合は nil です。
	Usage *gemini.TokenUsage
}
