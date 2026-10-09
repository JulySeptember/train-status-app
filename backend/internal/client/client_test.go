package client

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"train-status-app/backend/internal/model"
)

const testKey = "SECRET-KEY-123"

// newTestClient は、全ホストを1つのテスト用サーバーに向けた Client を返す。
// handler には、ホスト名（public・basic・challenge）をパスの先頭に付けたリクエストが届く。
func newTestClient(t *testing.T, sources []Source, key KeyFunc, handler http.HandlerFunc) *Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c := New(sources, key)
	c.baseURL = func(h Host) string { return srv.URL + "/" + h.String() }

	return c
}

func staticKey(_ context.Context, _ Host) (string, error) {
	return testKey, nil
}

// captureLog は、テストの間に log に出た内容を返す関数を返す。
func captureLog(t *testing.T) func() string {
	t.Helper()

	var mu sync.Mutex
	var buf bytes.Buffer

	prev := log.Writer()
	log.SetOutput(writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	}))
	t.Cleanup(func() { log.SetOutput(prev) })

	return func() string {
		mu.Lock()
		defer mu.Unlock()
		return buf.String()
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestGetTrainStatusQueries(t *testing.T) {

	var (
		mu       sync.Mutex
		requests []string
	)

	sources := []Source{Sources["Toei"], Sources["TokyoMetro"], Sources["JR-East"]}

	c := newTestClient(t, sources, staticKey, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.Path+" "+r.URL.Query().Get("odpt:operator")+" key="+r.URL.Query().Get("acl:consumerKey"))
		mu.Unlock()
		w.Write([]byte(`[{"odpt:railway": "odpt.Railway:` + r.URL.Query().Get("odpt:operator") + `"}]`))
	})

	got, err := c.GetTrainStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Items) != 3 || len(got.Succeeded) != 3 || len(got.Failed) != 0 {
		t.Fatalf("expected 3 statuses from 3 operators, got %+v", got)
	}

	slices.Sort(requests)
	want := []string{
		// JR東日本の運行情報は jre-is が配信する
		"/challenge/odpt:TrainInformation odpt.Operator:jre-is key=" + testKey,
		"/basic/odpt:TrainInformation odpt.Operator:TokyoMetro key=" + testKey,
		// 都営にはキーを付けない
		"/public/odpt:TrainInformation odpt.Operator:Toei key=",
	}
	slices.Sort(want)

	if !slices.Equal(requests, want) {
		t.Fatalf("expected %v, got %v", want, requests)
	}
}

// 列車位置は、配信している事業者にだけ問い合わせる。
func TestGetTrainLocationsOnlyLocationSources(t *testing.T) {

	var (
		mu        sync.Mutex
		operators []string
	)

	sources := []Source{Sources["Toei"], Sources["TokyoMetro"], Sources["Keio"]}

	c := newTestClient(t, sources, staticKey, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		operators = append(operators, r.URL.Query().Get("odpt:operator"))
		mu.Unlock()
		w.Write([]byte(`[]`))
	})

	if _, err := c.GetTrainLocations(context.Background()); err != nil {
		t.Fatal(err)
	}

	slices.Sort(operators)
	if want := []string{"odpt.Operator:Keio", "odpt.Operator:Toei"}; !slices.Equal(operators, want) {
		t.Fatalf("expected %v, got %v", want, operators)
	}
}

// 1つの事業者の列車位置は、その事業者にだけ問い合わせる。
func TestGetOperatorTrainLocations(t *testing.T) {

	var (
		mu        sync.Mutex
		operators []string
	)

	sources := []Source{Sources["Toei"], Sources["TokyoMetro"], Sources["Keio"]}

	c := newTestClient(t, sources, staticKey, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		operators = append(operators, r.URL.Query().Get("odpt:operator"))
		mu.Unlock()
		if strings.HasPrefix(r.URL.Path, "/public/") {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`[{"owl:sameAs": "odpt.Train:Keio.Keio.1"}]`))
	})

	got, err := c.GetOperatorTrainLocations(context.Background(), "Keio")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !slices.Equal(operators, []string{"odpt.Operator:Keio"}) {
		t.Fatalf("expected only the Keio request, got %+v %v", got, operators)
	}

	captureLog(t)

	// 失敗したら ErrExternalAPI
	if _, err := c.GetOperatorTrainLocations(context.Background(), "Toei"); !errors.Is(err, ErrExternalAPI) {
		t.Fatalf("expected ErrExternalAPI, got %v", err)
	}

	// 列車位置を配信していない事業者・設定していない事業者には問い合わせない
	for _, op := range []string{"TokyoMetro", "Tobu"} {
		if _, err := c.GetOperatorTrainLocations(context.Background(), op); !errors.Is(err, ErrNoSource) {
			t.Fatalf("%s: expected ErrNoSource, got %v", op, err)
		}
	}
	if len(operators) != 2 {
		t.Fatalf("unexpected requests %v", operators)
	}
}

func TestPartialFailure(t *testing.T) {

	logs := captureLog(t)

	sources := []Source{Sources["Toei"], Sources["TokyoMetro"]}

	c := newTestClient(t, sources, staticKey, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/basic/") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`[{"odpt:railway": "odpt.Railway:Toei.Asakusa"}]`))
	})

	got, err := c.GetTrainStatus(context.Background())
	if err != nil {
		t.Fatalf("expected the Toei result despite the Metro failure, got %v", err)
	}

	if len(got.Items) != 1 || got.Items[0].Railway != "odpt.Railway:Toei.Asakusa" {
		t.Fatalf("unexpected result %+v", got)
	}

	// 失敗した事業者を呼び出し側に知らせる（都営だけが失敗したときも、成功と見分けられるように）
	if !slices.Equal(got.Succeeded, []string{"Toei"}) || !slices.Equal(got.Failed, []string{"TokyoMetro"}) {
		t.Fatalf("expected Toei succeeded and TokyoMetro failed, got %v / %v", got.Succeeded, got.Failed)
	}

	if !strings.Contains(logs(), "TokyoMetro") || strings.Contains(logs(), testKey) {
		t.Fatalf("expected a log for TokyoMetro without the key, got %q", logs())
	}
}

func TestAllFailed(t *testing.T) {

	captureLog(t)

	c := newTestClient(t, []Source{Sources["Toei"]}, staticKey, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	})

	_, err := c.GetTrainStatus(context.Background())
	if !errors.Is(err, ErrExternalAPI) {
		t.Fatalf("expected ErrExternalAPI, got %v", err)
	}
}

// キーを取れない事業者は失敗として扱い、ほかの事業者の結果は返す。
func TestKeyError(t *testing.T) {

	logs := captureLog(t)

	noKey := func(_ context.Context, _ Host) (string, error) {
		return "", errors.New("not configured")
	}

	var requested []string
	var mu sync.Mutex

	c := newTestClient(t, []Source{Sources["Toei"], Sources["Keio"]}, noKey, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requested = append(requested, r.URL.Path)
		mu.Unlock()
		w.Write([]byte(`[]`))
	})

	got, err := c.GetTrainStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(got.Failed, []string{"Keio"}) {
		t.Fatalf("expected Keio to be failed, got %v", got.Failed)
	}

	if len(requested) != 1 || !strings.HasPrefix(requested[0], "/public/") {
		t.Fatalf("expected only the Toei request, got %v", requested)
	}

	if !strings.Contains(logs(), "Keio") {
		t.Fatalf("expected a log for Keio, got %q", logs())
	}
}

// 接続に失敗したとき、エラーとログにキー（URL）を出さない。
func TestTransportErrorDoesNotLeakKey(t *testing.T) {

	logs := captureLog(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // 接続できないサーバー

	c := New([]Source{Sources["TokyoMetro"]}, staticKey)
	c.baseURL = func(Host) string { return srv.URL }

	_, err := c.GetTrainStatus(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}

	if strings.Contains(err.Error(), testKey) || strings.Contains(logs(), testKey) {
		t.Fatalf("key leaked: err=%q log=%q", err, logs())
	}

	if !strings.Contains(logs(), "TokyoMetro") {
		t.Fatalf("expected a log for TokyoMetro, got %q", logs())
	}

	// 直接 fetchSource を呼んだときのエラーにも出さない
	_, err = fetchSource[model.TrainStatus](context.Background(), c, HostBasic, "odpt:Train", "TokyoMetro")
	if err == nil || strings.Contains(err.Error(), testKey) || strings.Contains(err.Error(), "consumerKey") {
		t.Fatalf("key leaked or no error: %v", err)
	}
}
