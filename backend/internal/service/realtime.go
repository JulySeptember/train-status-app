package service

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"train-status-app/backend/internal/client"
	"train-status-app/backend/internal/model"
	"train-status-app/backend/internal/route"
)

const (
	// realtimeTTL は、経路検索に使う運行状況（遅れと運転見合わせ）をキャッシュする時間
	realtimeTTL = 30 * time.Second

	// realtimeTimeout は運行状況の取得を待つ時間。超えたら時刻表どおりに探す
	realtimeTimeout = 3 * time.Second

	// realtimeRetryAfter は、運行状況を取得できなかったあと、取り直すまでの間隔。
	// そのあいだは取りに行かずに失敗を返す（ODPT の障害中に、検索ごとに realtimeTimeout ずつ待たせない）
	realtimeRetryAfter = 15 * time.Second

	// delayWindowMinutes は、現在から何分後までの時刻に遅れを足すか。
	// 遅れは現時点の値なので、先の列車には足さない
	delayWindowMinutes = 60

	// delayLookbackMinutes は、現在から何分前までに始発駅を出た列車に遅れを足すか。
	// それより前に出た列車は走り終えているとみなす（過ぎた時刻を指定した検索に、今の遅れを足さない）
	delayLookbackMinutes = 120
)

// 運行情報の文章に含まれていたら運転見合わせとみなす言葉。
// 平常時の文章は「現在、１５分以上の遅延はありません。」で、状態を表す項目は無い
var suspendedWords = []string{"見合わせ", "運転を中止", "運転中止"}

// suspendedWords のすぐ後に続いていたら、終わった見合わせ（「見合わせていましたが、運転を再開しました」）とみなす言い方。
// 「再開」の有無では判定しない。「運転再開は15時頃を見込んでいます」「一部の区間は運転を再開し」のように、
// 見合わせが続いている文章にも入るため
var pastSuffixes = []string{
	"ていまし", "ておりまし", "ていた", "まし", "た", // 見合わせていましたが・見合わせましたが・見合わせた
	"していまし", "しておりまし", "していた", "しまし", "した", // 運転を中止していましたが・中止しましたが
	"となっていまし", "となっておりまし", "となりまし", "を行っていまし", "を行っておりまし", "をしていまし", // 運転見合わせとなっていましたが
}

// suspendedWords のすぐ前にあったら見合わせとみなさない言葉。
// 「JR線との直通運転を中止して、りんかい線内で折返し運転」は、路線の中では走っている
var throughServicePrefixes = []string{"直通", "直通運転を", "直通運転の"}

// railwayConditions は、経路検索に反映する路線ごとの運行状況。
type railwayConditions struct {
	// 路線・方向ごとの遅れ（1分以上のものだけ）
	delays []route.Delay

	// 運転を見合わせている路線の ID
	suspended []string

	// 路線ID → 運行情報の文章
	texts map[string]string

	// 運行情報を取得できた事業者（client.Source の Name）。
	// 含まれない事業者の路線は、文章が無くても平常とは限らない
	statusOperators map[string]bool

	// 列車位置を取得できた事業者（client.Source の Name）
	locationOperators map[string]bool
}

type realtimeCache struct {
	mu    sync.Mutex
	at    time.Time
	value *railwayConditions

	// 前回の取得の失敗と、その時刻
	err      error
	failedAt time.Time
}

// railwayConditions は運行状況を返す。取得した結果は realtimeTTL のあいだ使い回す。
// 同時に呼ばれたときは、先に呼ばれた取得の結果を待つ。
// 取得に失敗したら、realtimeRetryAfter のあいだは取りに行かずに同じエラーを返す。
func (s *Service) railwayConditions(ctx context.Context) (*railwayConditions, error) {

	s.realtime.mu.Lock()
	defer s.realtime.mu.Unlock()

	now := s.now()
	if s.realtime.value != nil && now.Sub(s.realtime.at) < realtimeTTL {
		return s.realtime.value, nil
	}

	if s.realtime.err != nil && now.Sub(s.realtime.failedAt) < realtimeRetryAfter {
		return nil, s.realtime.err
	}

	fetchCtx, cancel := context.WithTimeout(ctx, realtimeTimeout)
	defer cancel()

	var (
		wg       sync.WaitGroup
		statuses client.Result[model.TrainStatus]
		trains   client.Result[model.TrainLocation]
		errs     [2]error
	)

	wg.Go(func() { statuses, errs[0] = s.client.GetTrainStatus(fetchCtx) })
	wg.Go(func() { trains, errs[1] = s.client.GetTrainLocations(fetchCtx) })
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			// 呼び出し側が待てなくなった（期限切れ・キャンセル）だけなら、次の呼び出しですぐ取り直す
			if ctx.Err() == nil {
				s.realtime.err = err
				s.realtime.failedAt = now
			}
			return nil, err
		}
	}

	value := &railwayConditions{
		delays:            medianDelays(trains.Items),
		texts:             s.statusTexts(statuses.Items),
		statusOperators:   make(map[string]bool, len(statuses.Succeeded)),
		locationOperators: make(map[string]bool, len(trains.Succeeded)),
	}

	for _, name := range trains.Succeeded {
		value.locationOperators[name] = true
	}

	for _, name := range statuses.Succeeded {
		value.statusOperators[name] = true
	}

	// 見合わせは路線ごとの運行情報だけで判定する。会社全体で1件の運行情報（京急・西武）は、
	// 1路線の見合わせでも全路線を見合わせにしてしまうので、表示にだけ使う
	for _, st := range statuses.Items {
		if _, ok := s.railwayNames[st.Railway]; ok && isSuspended(st.TrainInformationText.Ja) &&
			!slices.Contains(value.suspended, st.Railway) {
			value.suspended = append(value.suspended, st.Railway)
		}
	}
	slices.Sort(value.suspended)

	s.realtime.at = now
	s.realtime.value = value
	s.realtime.err = nil

	return value, nil
}

// statusTexts は、運行情報を路線ID → 文章にする。
//   - 会社全体で1件の運行情報（京急・西武。odpt:railway が無い）は、その会社の全路線に当てはめる。
//     路線ごとの運行情報があれば、そちらを使う
//   - 対象の路線（assets の路線）だけを残す。JR東日本・東武などは都外の路線の運行情報も配信している
func (s *Service) statusTexts(statuses []model.TrainStatus) map[string]string {

	result := make(map[string]string, len(statuses))

	for _, st := range statuses {
		if _, ok := s.railwayNames[st.Railway]; ok {
			result[st.Railway] = st.TrainInformationText.Ja
		}
	}

	for _, st := range statuses {
		if st.Railway != "" {
			continue
		}
		for _, id := range s.operatorRailways[operatorOf(st.Operator)] {
			if _, ok := result[id]; !ok {
				result[id] = st.TrainInformationText.Ja
			}
		}
	}

	return result
}

// isSuspended は、文章のどこかで運転見合わせが続いているかを返す。
func isSuspended(text string) bool {
	for _, word := range suspendedWords {
		rest := text
		for {
			i := strings.Index(rest, word)
			if i < 0 {
				break
			}
			before := rest[:i]
			through := slices.ContainsFunc(throughServicePrefixes, func(p string) bool { return strings.HasSuffix(before, p) })
			rest = rest[i+len(word):]
			if !through && !slices.ContainsFunc(pastSuffixes, func(p string) bool { return strings.HasPrefix(rest, p) }) {
				return true
			}
		}
	}
	return false
}

// medianDelays は、路線・方向ごとに列車の遅れ（odpt:delay、秒）の中央値を分にして返す。
// 遅れの値が無い路線（荒川線は null で配信される）は 0 になり、結果に含めない。
func medianDelays(trains []model.TrainLocation) []route.Delay {

	type key struct{ railway, direction string }

	groups := make(map[key][]int)
	for _, t := range trains {
		k := key{t.Railway, t.RailDirection}
		groups[k] = append(groups[k], t.Delay)
	}

	var result []route.Delay

	keys := slices.SortedFunc(maps.Keys(groups), func(a, b key) int {
		return cmp.Or(cmp.Compare(a.railway, b.railway), cmp.Compare(a.direction, b.direction))
	})

	for _, k := range keys {
		delays := groups[k]
		slices.Sort(delays)

		minutes := (delays[len(delays)/2] + 30) / 60
		if minutes <= 0 {
			continue
		}

		result = append(result, route.Delay{
			Railway:       k.railway,
			RailDirection: k.direction,
			Minutes:       minutes,
		})
	}

	return result
}
