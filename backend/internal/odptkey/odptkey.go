// Package odptkey は、ODPT のキー（api.odpt.org・api-challenge.odpt.org 用）を返す。
//
// 手元では環境変数（backend/.env）の値を使う。Lambda ではキーを環境変数に置かず、
// SSM Parameter Store（SecureString）から、初めて要るときに読んでプロセスが続く間は持ち続ける
// （AI のキーと同じ。docs/design/multi-operator.md 4.3）。
// キーの値はログやエラーに出さない。
package odptkey

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"train-status-app/backend/internal/client"
)

const (
	// 読むのに失敗したとき（未登録を含む）に、次に試すまでの間隔。
	// 登録すれば再デプロイせずに使えるようにする
	retryInterval = time.Minute

	loadTimeout = 3 * time.Second
)

// ErrNotConfigured は、そのホストのキーが設定されていないこと。
var ErrNotConfigured = errors.New("odpt key is not configured")

// errUnavailable は、前回キーを読めず、まだ読み直す時刻になっていないこと。
var errUnavailable = errors.New("odpt key is temporarily unavailable")

// Static は、環境変数などで渡されたキーを返す（手元用）。
func Static(basic, challenge string) client.KeyFunc {
	return func(_ context.Context, host client.Host) (string, error) {
		var key string
		switch host {
		case client.HostBasic:
			key = basic
		case client.HostChallenge:
			key = challenge
		}
		if key == "" {
			return "", fmt.Errorf("%s: %w", host, ErrNotConfigured)
		}
		return key, nil
	}
}

// loader は、SSM のパラメータを読む。テストで差し替える。
type loader func(ctx context.Context, name string) (string, error)

type cached struct {
	mu      sync.Mutex
	load    loader
	names   map[client.Host]string
	values  map[client.Host]string
	retryAt map[client.Host]time.Time
	now     func() time.Time
}

// SSM は、SSM Parameter Store から読んだキーを返す（Lambda 用）。
// basicParam・challengeParam はパラメータ名で、空ならそのホストのキーは設定されていない。
func SSM(basicParam, challengeParam string) client.KeyFunc {

	var (
		once   sync.Once
		ssmCli *ssm.Client
		cfgErr error
	)

	load := func(ctx context.Context, name string) (string, error) {
		once.Do(func() {
			var c aws.Config
			c, cfgErr = config.LoadDefaultConfig(ctx)
			if cfgErr == nil {
				ssmCli = ssm.NewFromConfig(c)
			}
		})
		if cfgErr != nil {
			return "", cfgErr
		}

		out, err := ssmCli.GetParameter(ctx, &ssm.GetParameterInput{
			Name:           aws.String(name),
			WithDecryption: aws.Bool(true),
		})
		var notFound *ssmtypes.ParameterNotFound
		if errors.As(err, &notFound) {
			return "", fmt.Errorf("parameter %s is not registered", name)
		}
		if err != nil {
			return "", err
		}
		return aws.ToString(out.Parameter.Value), nil
	}

	return newCached(load, basicParam, challengeParam, time.Now).key
}

func newCached(load loader, basicParam, challengeParam string, now func() time.Time) *cached {
	return &cached{
		load: load,
		names: map[client.Host]string{
			client.HostBasic:     basicParam,
			client.HostChallenge: challengeParam,
		},
		values:  make(map[client.Host]string),
		retryAt: make(map[client.Host]time.Time),
		now:     now,
	}
}

func (c *cached) key(ctx context.Context, host client.Host) (string, error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	if v, ok := c.values[host]; ok {
		return v, nil
	}

	name := c.names[host]
	if name == "" {
		return "", fmt.Errorf("%s: %w", host, ErrNotConfigured)
	}

	if c.now().Before(c.retryAt[host]) {
		return "", fmt.Errorf("%s: %w", host, errUnavailable)
	}

	ctx, cancel := context.WithTimeout(ctx, loadTimeout)
	defer cancel()

	v, err := c.load(ctx, name)
	if err == nil && v == "" {
		err = errors.New("empty value")
	}
	if err != nil {
		c.retryAt[host] = c.now().Add(retryInterval)
		return "", fmt.Errorf("%s: %w", host, err)
	}

	c.values[host] = v

	return v, nil
}
