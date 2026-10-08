package ai

// Model は使えるモデル。Phase 4 のモデル選択でも使う
type Model struct {
	ID       string
	Name     string
	Provider string
	FreeTier bool
}

// Models は使えるモデルの一覧。先頭が初期値。
// 2026-10 時点で、Google が新規に推奨し、無料枠のあるモデル（https://ai.google.dev/gemini-api/docs/models）。
// 軽量な Flash-Lite で評価セット（testdata/eval.yaml）を試し、手順の判断が安定しなければ Flash にする（設計書 6.3）。
var Models = []Model{
	{ID: "gemini-3.5-flash-lite", Name: "Gemini 3.5 Flash-Lite", Provider: "gemini", FreeTier: true},
	{ID: "gemini-3.8-flash", Name: "Gemini 3.8 Flash", Provider: "gemini", FreeTier: true},
}

// ResolveModel は、設定（環境変数 AI_MODEL）があればそれを、無ければ一覧の先頭を返す。
// 一覧に無いモデルも指定できる（新しいモデルを試すため）。存在しなければ ErrInvalidModel になる。
func ResolveModel(configured string) string {
	if configured != "" {
		return configured
	}
	return Models[0].ID
}
