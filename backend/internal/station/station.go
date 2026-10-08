// Package station は、ユーザーや AI が書いた駅名から駅を特定する。
package station

import (
	"slices"
	"strings"
	"unicode"

	"train-status-app/backend/internal/model"
)

// maxCandidates は、部分一致で返す候補の上限
const maxCandidates = 10

// Station は駅名の候補。同じ名前の駅は路線ごとに別の駅として返す
type Station struct {
	ID      string
	Name    string
	Railway string
}

type entry struct {
	Station
	ja string // 正規化した日本語名
	en string // 正規化した英語名
}

// Index は駅名から駅を引くための索引。
type Index struct {
	entries []entry
}

func New(stations []model.Station) *Index {
	idx := &Index{}
	for _, st := range stations {
		idx.entries = append(idx.entries, entry{
			Station: Station{
				ID:      st.SameAs,
				Name:    st.StationTitle.Ja,
				Railway: st.Railway,
			},
			ja: Normalize(st.StationTitle.Ja),
			en: Normalize(st.StationTitle.En),
		})
	}
	return idx
}

// Find は name に合う駅を返す。完全に一致する駅（同じ名前の駅は路線ごとに複数）があればそれだけを返し、
// 無ければ名前の一部に一致する駅を最大 maxCandidates 件返す。どれにも合わなければ空。
func (idx *Index) Find(name string) []Station {
	q := Normalize(name)
	if q == "" {
		return nil
	}

	var exact, partial []Station
	for _, e := range idx.entries {
		switch {
		case e.ja == q || e.en == q:
			exact = append(exact, e.Station)
		case strings.Contains(e.ja, q) || (len(q) >= 3 && strings.Contains(e.en, q)):
			partial = append(partial, e.Station)
		}
	}

	if len(exact) > 0 {
		return exact
	}

	// 名前が短いほど入力に近いので、文字数の少ない順に並べる
	slices.SortStableFunc(partial, func(a, b Station) int {
		return len([]rune(a.Name)) - len([]rune(b.Name))
	})

	if len(partial) > maxCandidates {
		partial = partial[:maxCandidates]
	}
	return partial
}

// Normalize は駅名の表記の揺れをそろえる。
// 全角の英数字を半角に、英字を小文字にし、空白・記号を除き、末尾の「駅」を取り、「ケ」「ヵ」を「ヶ」にする。
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range s {
		// 全角の英数字・記号（！〜～）を半角にする
		if r >= '！' && r <= '～' {
			r -= 0xFEE0
		}

		switch {
		case unicode.IsSpace(r), r == '-', r == '・', r == '.', r == '\'':
			continue
		case r == 'ケ' || r == 'ヵ':
			r = 'ヶ'
		}

		b.WriteRune(unicode.ToLower(r))
	}

	return strings.TrimSuffix(b.String(), "駅")
}
