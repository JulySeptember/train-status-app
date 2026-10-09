package service

import (
	"context"
	"slices"
	"testing"

	"train-status-app/backend/assets"
	"train-status-app/backend/internal/model"
)

func TestOperatorOf(t *testing.T) {
	tests := map[string]string{
		"odpt.Railway:Toei.Asakusa":             "Toei",
		"odpt.Train:JR-East.Yamanote.1234G":     "JR-East",
		"odpt.Operator:Keikyu":                  "Keikyu",
		"odpt.Station:TokyoMetro.Ginza.Shibuya": "TokyoMetro",
		"":                                      "",
	}
	for id, want := range tests {
		if got := operatorOf(id); got != want {
			t.Errorf("%q: expected %q, got %q", id, want, got)
		}
	}
}

func TestLocationAvailable(t *testing.T) {
	tests := map[string]bool{
		"odpt.Railway:Toei.Mita":             true,
		"odpt.Railway:Toei.NipporiToneri":    false,
		"odpt.Railway:JR-East.Yamanote":      true,
		"odpt.Railway:TokyoMetro.Ginza":      false, // メトロは odpt:Train を配信していない
		"odpt.Railway:Tokyu.Toyoko":          false,
		"odpt.Railway:Yurikamome.Yurikamome": false, // client.Sources に無い事業者
	}
	for id, want := range tests {
		if got := locationAvailable(id); got != want {
			t.Errorf("%s: expected %v, got %v", id, want, got)
		}
	}

	if delayAvailable("odpt.Railway:Keikyu.Main") || delayAvailable("odpt.Railway:Toei.Arakawa") || !delayAvailable("odpt.Railway:Keio.Keio") {
		t.Error("unexpected delayAvailable")
	}
}

func newOperatorService(t *testing.T, c *mockClient) *Service {
	t.Helper()
	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}
	return New(c, loader)
}

// 会社全体で1件の運行情報は全路線に当てはめ、対象外の路線の運行情報は捨てる。
func TestGetTrainStatusOperatorWide(t *testing.T) {

	const mita = "odpt.Railway:Toei.Mita"

	s := newOperatorService(t, &mockClient{
		trainStatus: []model.TrainStatus{
			{Operator: "odpt.Operator:Toei", TrainInformationText: model.LocalizedString{Ja: "各線平常"}},
			{Railway: mita, Operator: "odpt.Operator:Toei", TrainInformationText: model.LocalizedString{Ja: "遅延"}},
			// 都外の路線（JR東日本・東武などが配信する）
			{Railway: "odpt.Railway:Toei.Unknown", Operator: "odpt.Operator:Toei", TrainInformationText: model.LocalizedString{Ja: "遅延"}},
		},
	})

	got, err := s.GetTrainStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(s.assets.Railways()) {
		t.Fatalf("expected one status per railway, got %+v", got)
	}

	for i, st := range got {
		if st.RailwayID != s.assets.Railways()[i].SameAs {
			t.Errorf("expected railway order, got %s at %d", st.RailwayID, i)
		}
		want := "各線平常"
		if st.RailwayID == mita {
			want = "遅延" // 路線ごとの運行情報を優先する
		}
		if st.Status != want || st.Unavailable {
			t.Errorf("%s: expected %q, got %+v", st.RailwayID, want, st)
		}
	}
}

// 事業者の取得に失敗したら、その事業者の路線を unavailable で返す（平常と見分けられるように）。
func TestGetTrainStatusFailedOperator(t *testing.T) {

	s := newOperatorService(t, &mockClient{
		statusSucceeded: []string{"TokyoMetro"},
		statusFailed:    []string{"Toei"},
	})

	got, err := s.GetTrainStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(s.assets.Railways()) {
		t.Fatalf("expected all Toei railways, got %+v", got)
	}

	for _, st := range got {
		if !st.Unavailable || st.Status != statusUnavailableText {
			t.Errorf("expected unavailable, got %+v", st)
		}
	}

	// 失敗していない事業者で、運行情報の無い路線は返さない（今までどおり）
	s = newOperatorService(t, &mockClient{})
	if got, err := s.GetTrainStatus(context.Background()); err != nil || len(got) != 0 {
		t.Fatalf("expected no statuses, got %+v %v", got, err)
	}
}

// 列車位置は、列車の事業者にだけ問い合わせる。
func TestGetTrainLocationQueriesOperator(t *testing.T) {

	c := &mockClient{}
	s := newOperatorService(t, c)

	trainID, _ := findTrain(t, s.assets, "odpt.Railway:Toei.Mita")

	if _, err := s.GetTrainLocation(context.Background(), trainID); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(c.locationOperators, []string{"Toei"}) {
		t.Fatalf("expected only Toei to be queried, got %v", c.locationOperators)
	}
}

// 事業者を設定していない（ODPT_OPERATORS に無い）ときは、位置が無いと案内する。
func TestGetTrainLocationNoSource(t *testing.T) {

	s := newOperatorService(t, &mockClient{locNoSource: true})

	trainID, number := findTrain(t, s.assets, "odpt.Railway:Toei.Mita")

	got, err := s.GetTrainLocation(context.Background(), trainID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Available || got.NotRunning != "" || got.TrainNumber != number || got.Message == "" {
		t.Fatalf("expected unsupported, got %+v", got)
	}
}

// 運行情報を取得できなかった事業者の路線は unknown にする。
func TestGetRailwayConditionsUnknown(t *testing.T) {

	s := newOperatorService(t, &mockClient{
		statusSucceeded: []string{},
		statusFailed:    []string{"Toei"},
		trainLocations:  mitaDelayed(300),
	})

	got, err := s.GetRailwayConditions(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range got {
		want := RailwayUnknown
		if c.Railway == mitaRailway {
			want = RailwayDelayed // 列車位置の遅れは分かる
		}
		if c.State != want {
			t.Errorf("%s: expected %s, got %s", c.Railway, want, c.State)
		}
	}
}

// 会社全体で1件の運行情報は、見合わせの判定に使わない（1路線の見合わせで全路線を避けないように）。
func TestOperatorWideStatusIsNotSuspension(t *testing.T) {

	s := newOperatorService(t, &mockClient{
		trainStatus: []model.TrainStatus{
			{Operator: "odpt.Operator:Toei", TrainInformationText: model.LocalizedString{Ja: "三田線で運転を見合わせています。"}},
		},
	})

	conds, err := s.railwayConditions(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(conds.suspended) != 0 {
		t.Fatalf("expected no suspended railways, got %v", conds.suspended)
	}

	// 文章は各路線に表示する
	if conds.texts[mitaRailway] == "" {
		t.Fatal("expected the operator-wide text for each railway")
	}
}
