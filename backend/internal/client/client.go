// Package client は、ODPT のリアルタイムの情報（運行情報 odpt:TrainInformation と
// 列車位置 odpt:Train）を事業者ごとに取得する。
//
// 都営はキー不要の api-public.odpt.org から、それ以外はキーの要る api.odpt.org（基本ライセンス）・
// api-challenge.odpt.org（チャレンジ限定ライセンス）から取る（docs/design/multi-operator.md 9章）。
// キーは URL のクエリに入るので、エラーやログに URL を出さない。
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"train-status-app/backend/internal/model"
)

var ErrExternalAPI = errors.New("external api error")

// ErrNoSource は、問い合わせる事業者が無いこと（設定していない、または配信していない）。
var ErrNoSource = errors.New("no source for the operator")

// maxRecords は、ODPT の API が1回に返す件数の上限。
const maxRecords = 1000

// Host は ODPT の API のホスト。ライセンスとキーがホストごとに違う。
type Host int

const (
	// HostPublic は api-public.odpt.org（CC BY 4.0 など。キー不要）
	HostPublic Host = iota

	// HostBasic は api.odpt.org（公共交通オープンデータ基本ライセンス）
	HostBasic

	// HostChallenge は api-challenge.odpt.org（チャレンジ限定ライセンス）
	HostChallenge
)

func (h Host) baseURL() string {
	switch h {
	case HostBasic:
		return "https://api.odpt.org/api/v4"
	case HostChallenge:
		return "https://api-challenge.odpt.org/api/v4"
	default:
		return "https://api-public.odpt.org/api/v4"
	}
}

func (h Host) String() string {
	switch h {
	case HostBasic:
		return "basic"
	case HostChallenge:
		return "challenge"
	default:
		return "public"
	}
}

// Source は、リアルタイムの情報を取る事業者1つ。
type Source struct {
	// 事業者の名前（odpt.Operator:<Name>）
	Name string

	Host Host

	// 運行情報の事業者。JR東日本の運行情報は JR東日本アイステイションズ（jre-is）が配信している
	StatusOperator string

	// 運行情報を配信しているか（小田急・ゆりかもめは無い）
	Status bool

	// 列車位置（odpt:Train）を配信しているか
	Location bool
}

// Sources は、対応している事業者（docs/design/multi-operator.md 2.2）。
var Sources = map[string]Source{
	"Toei": {Name: "Toei", Host: HostPublic, Status: true, Location: true},

	"TokyoMetro":   {Name: "TokyoMetro", Host: HostBasic, Status: true},
	"TWR":          {Name: "TWR", Host: HostBasic, Status: true},
	"MIR":          {Name: "MIR", Host: HostBasic, Status: true},
	"TamaMonorail": {Name: "TamaMonorail", Host: HostBasic, Status: true},

	"JR-East": {Name: "JR-East", Host: HostChallenge, StatusOperator: "jre-is", Status: true, Location: true},
	"Keio":    {Name: "Keio", Host: HostChallenge, Status: true, Location: true},
	"Tobu":    {Name: "Tobu", Host: HostChallenge, Status: true, Location: true},
	"Keikyu":  {Name: "Keikyu", Host: HostChallenge, Status: true, Location: true},
	"Tokyu":   {Name: "Tokyu", Host: HostChallenge, Status: true},
	"Seibu":   {Name: "Seibu", Host: HostChallenge, Status: true},
}

// KeyFunc は、ホストのキーを返す。HostPublic では呼ばない。
type KeyFunc func(ctx context.Context, host Host) (string, error)

type Client struct {
	http    *http.Client
	baseURL func(Host) string
	sources []Source
	key     KeyFunc
}

// New は、sources の事業者からリアルタイムの情報を取る Client を返す。
func New(sources []Source, key KeyFunc) *Client {
	return &Client{
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: Host.baseURL,
		sources: sources,
		key:     key,
	}
}

// Result は、事業者ごとに取得した結果をまとめたもの。
type Result[T any] struct {
	Items []T

	// 取得できた事業者と、取得に失敗した事業者（Source.Name。問い合わせた順）
	Succeeded []string
	Failed    []string
}

// GetTrainStatus は、運行情報を全事業者から取る。
func (c *Client) GetTrainStatus(ctx context.Context) (Result[model.TrainStatus], error) {
	return fetchAll[model.TrainStatus](ctx, c, c.sources, "odpt:TrainInformation", func(s Source) (string, bool) {
		operator := s.StatusOperator
		if operator == "" {
			operator = s.Name
		}
		return operator, s.Status
	})
}

// GetTrainLocations は、列車位置を全事業者から取る。
func (c *Client) GetTrainLocations(ctx context.Context) (Result[model.TrainLocation], error) {
	return fetchAll[model.TrainLocation](ctx, c, c.sources, "odpt:Train", locationTarget)
}

// GetOperatorTrainLocations は、1つの事業者（odpt.Operator:<Name> の Name）の列車位置だけを取る。
// その事業者を設定していないか、列車位置を配信していなければ ErrNoSource を返す。
func (c *Client) GetOperatorTrainLocations(ctx context.Context, operator string) ([]model.TrainLocation, error) {

	i := slices.IndexFunc(c.sources, func(s Source) bool { return s.Name == operator && s.Location })
	if i < 0 {
		return nil, ErrNoSource
	}

	result, err := fetchAll[model.TrainLocation](ctx, c, c.sources[i:i+1], "odpt:Train", locationTarget)
	if err != nil {
		return nil, err
	}

	return result.Items, nil
}

func locationTarget(s Source) (string, bool) {
	return s.Name, s.Location
}

// fetchAll は、各事業者から並列に取得してまとめる。
// 一部の事業者が失敗しても、残りの結果を返す（失敗はログに出し、Result.Failed に入れる）。
// 全部失敗したときだけエラーを返す。
func fetchAll[T any](
	ctx context.Context,
	c *Client,
	sources []Source,
	endpoint string,
	target func(Source) (operator string, ok bool),
) (Result[T], error) {

	type result struct {
		data []T
		err  error
	}

	var (
		wg      sync.WaitGroup
		results = make([]*result, len(sources))
	)

	for i, s := range sources {
		operator, ok := target(s)
		if !ok {
			continue
		}
		results[i] = &result{}
		wg.Go(func() {
			results[i].data, results[i].err = fetchSource[T](ctx, c, s.Host, endpoint, operator)
		})
	}

	wg.Wait()

	var out Result[T]

	for i, r := range results {
		if r == nil {
			continue
		}
		name := sources[i].Name
		if r.err != nil {
			out.Failed = append(out.Failed, name)
			log.Printf("odpt %s %s: %v", endpoint, name, r.err)
			continue
		}
		// ODPT の API は1回に 1,000 件までしか返さない。切れていても分からないので知らせる
		if len(r.data) >= maxRecords {
			log.Printf("odpt %s %s: %d records, the result may be truncated", endpoint, name, len(r.data))
		}
		out.Succeeded = append(out.Succeeded, name)
		out.Items = append(out.Items, r.data...)
	}

	if len(out.Failed) > 0 && len(out.Succeeded) == 0 {
		return Result[T]{}, ErrExternalAPI
	}

	return out, nil
}

func fetchSource[T any](ctx context.Context, c *Client, host Host, endpoint, operator string) ([]T, error) {

	query := url.Values{"odpt:operator": {"odpt.Operator:" + operator}}

	if host != HostPublic {
		key, err := c.key(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("%s key: %w", host, err)
		}
		query.Set("acl:consumerKey", key)
	}

	return fetch[T](ctx, c.http, c.baseURL(host)+"/"+endpoint+"?"+query.Encode())
}

func fetch[T any](
	ctx context.Context,
	httpClient *http.Client,
	rawURL string,
) ([]T, error) {

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		rawURL,
		nil,
	)
	if err != nil {
		return nil, errors.New("invalid request")
	}

	res, err := httpClient.Do(req)
	if err != nil {
		// *url.Error のメッセージには URL（キーを含む）が入るので、元のエラーだけを返す
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return nil, urlErr.Err
		}
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrExternalAPI, res.StatusCode)
	}

	var data []T

	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}
