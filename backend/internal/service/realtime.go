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

	// delayWindowMinutes は、現在から何分後までの時刻に遅れを足すか。
	// 遅れは現時点の値なので、先の列車には足さない
	delayWindowMinutes = 60
)

// 運行情報の文章に含まれていたら運転見合わせとみなす言葉。
// 平常時の文章は「現在、１５分以上の遅延はありません。」で、状態を表す項目は無い
var suspendedWords = []string{"見合わせ", "運転を中止", "運転中止"}

// 「見合わせていましたが、運転を再開しました」のような文章は見合わせとみなさない
var resumedWords = []string{"再開"}

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
}

type realtimeCache struct {
	mu    sync.Mutex
	at    time.Time
	value *railwayConditions
}

// railwayConditions は運行状況を返す。取得した結果は realtimeTTL のあいだ使い回す。
// 同時に呼ばれたときは、先に呼ばれた取得の結果を待つ。
func (s *Service) railwayConditions(ctx context.Context) (*railwayConditions, error) {

	s.realtime.mu.Lock()
	defer s.realtime.mu.Unlock()

	now := s.now()
	if s.realtime.value != nil && now.Sub(s.realtime.at) < realtimeTTL {
		return s.realtime.value, nil
	}

	ctx, cancel := context.WithTimeout(ctx, realtimeTimeout)
	defer cancel()

	var (
		wg       sync.WaitGroup
		statuses client.Result[model.TrainStatus]
		trains   client.Result[model.TrainLocation]
		errs     [2]error
	)

	wg.Go(func() { statuses, errs[0] = s.client.GetTrainStatus(ctx) })
	wg.Go(func() { trains, errs[1] = s.client.GetTrainLocations(ctx) })
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	value := &railwayConditions{
		delays:          medianDelays(trains.Items),
		texts:           s.statusTexts(statuses.Items),
		statusOperators: make(map[string]bool, len(statuses.Succeeded)),
	}

	for _, name := range statuses.Succeeded {
		value.statusOperators[name] = true
	}

	for _, id := range slices.Sorted(maps.Keys(value.texts)) {
		if isSuspended(value.texts[id]) {
			value.suspended = append(value.suspended, id)
		}
	}

	s.realtime.at = now
	s.realtime.value = value

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

func isSuspended(text string) bool {
	contains := func(word string) bool { return strings.Contains(text, word) }
	return slices.ContainsFunc(suspendedWords, contains) &&
		!slices.ContainsFunc(resumedWords, contains)
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
