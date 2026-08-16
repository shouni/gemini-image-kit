package ports

import (
	"context"
	"errors"

	"github.com/shouni/go-gemini-client/gemini"
)

// ErrResolverNotApplicable は、ReferenceResolver がその参照を扱えない場合に返します。
//
// 「解決に失敗した」ではなく「管轄外」を表します。ResolverChain はこのエラーだけを
// 次の resolver へ進む合図として扱い、それ以外のエラーはその場で返します。取得失敗と
// 管轄外を同じ扱いにすると、ネットワーク障害が黙って別経路へ流れて原因が消えます。
var ErrResolverNotApplicable = errors.New("imagekit: resolver does not handle this reference")

// ReferenceResolver は、参照画像 1 件をモデルへ送れる添付へ解決します。
//
// 「どう送るか」（GCS URI をそのまま参照 / File API へ上げて URI 参照 / 取得して
// インライン）はバックエンドと運用で変わるため、キットが内部で決め打ちせず、
// 利用側が実装を選んで注入します。gs:// しか使わない構成なら取得・アップロード・
// キャッシュの依存を一切渡さずに済みます。
//
// 参照解決は複数画像に対して並行に走るため、実装は同時アクセス安全である必要があります。
type ReferenceResolver interface {
	// Resolve は参照を添付へ変換します。扱えない参照には ErrResolverNotApplicable を
	// 返してください（エラーではなく辞退の意思表示です）。
	Resolve(ctx context.Context, uri ImageURI) (gemini.Attachment, error)
}
