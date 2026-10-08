// Package awssetup は、Lambda で AI エージェントを組み立てる。
// API キーは SSM Parameter Store（SecureString）から、利用上限は DynamoDB で数える（設計書 9章・10.1）。
// AI 以外の API のコールドスタートを遅くしないよう、/api/chat が初めて呼ばれたときに準備する。
package awssetup

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"train-status-app/backend/internal/ai"
	"train-status-app/backend/internal/ai/dynamolimit"
	"train-status-app/backend/internal/ai/gemini"
)

const (
	// 準備にかける時間の上限
	setupTimeout = 5 * time.Second

	// キーがまだ登録されていないときに、次に確かめるまでの間隔。登録すれば再デプロイせずに使えるようにする
	notRegisteredRetry = time.Minute

	// 準備に失敗したときに、次に試すまでの間隔
	failedRetry = 10 * time.Second
)

// errNotRegistered は、SSM に API キーが登録されていないこと
var errNotRegistered = errors.New("api key parameter is not registered")

type Config struct {
	// API キーを入れた SSM のパラメータ名（SecureString）
	KeyParameter string

	// 利用上限を数える DynamoDB のテーブル名
	UsageTable string

	Model  string
	Limits ai.Limits
}

type deps struct {
	loadKey    func(ctx context.Context) (string, error)
	newLimiter func(ctx context.Context) (ai.Limiter, error)
	newChat    func(key string, limiter ai.Limiter) *ai.Service
}

// Chat は、初めて呼ばれたときに AI エージェントを組み立てる ChatService。
type Chat struct {
	deps deps
	now  func() time.Time

	mu      sync.Mutex
	chat    *ai.Service
	retryAt time.Time
}

func New(cfg Config, tools ai.ToolSet) *Chat {

	var (
		once   sync.Once
		awsCfg aws.Config
		cfgErr error
	)
	loadAWS := func(ctx context.Context) (aws.Config, error) {
		once.Do(func() { awsCfg, cfgErr = config.LoadDefaultConfig(ctx) })
		return awsCfg, cfgErr
	}

	return &Chat{
		now: time.Now,
		deps: deps{
			loadKey: func(ctx context.Context) (string, error) {
				c, err := loadAWS(ctx)
				if err != nil {
					return "", err
				}
				out, err := ssm.NewFromConfig(c).GetParameter(ctx, &ssm.GetParameterInput{
					Name:           aws.String(cfg.KeyParameter),
					WithDecryption: aws.Bool(true),
				})
				var notFound *ssmtypes.ParameterNotFound
				if errors.As(err, &notFound) {
					return "", errNotRegistered
				}
				if err != nil {
					return "", err
				}
				return aws.ToString(out.Parameter.Value), nil
			},
			newLimiter: func(ctx context.Context) (ai.Limiter, error) {
				c, err := loadAWS(ctx)
				if err != nil {
					return nil, err
				}
				return dynamolimit.New(dynamodb.NewFromConfig(c), cfg.UsageTable, cfg.Limits), nil
			},
			newChat: func(key string, limiter ai.Limiter) *ai.Service {
				return ai.NewService(gemini.New(key), ai.ResolveModel(cfg.Model), tools, limiter, ai.DefaultConfig())
			},
		},
	}
}

func (c *Chat) Chat(ctx context.Context, clientIP string, req ai.ChatRequest) (*ai.ChatResponse, error) {

	svc, err := c.service(ctx)
	if err != nil {
		return nil, err
	}

	res, err := svc.Chat(ctx, clientIP, req)

	// キーが無効になった（差し替えた）ときは、次の質問でキーを読み直す
	if errors.Is(err, ai.ErrAuth) {
		c.mu.Lock()
		if c.chat == svc {
			c.chat = nil
		}
		c.mu.Unlock()
	}

	return res, err
}

// service は、組み立て済みのエージェントを返す。まだなら組み立てる
func (c *Chat) service(ctx context.Context) (*ai.Service, error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.chat != nil {
		return c.chat, nil
	}

	now := c.now()
	if now.Before(c.retryAt) {
		return nil, fmt.Errorf("%w: ai agent is not ready", ai.ErrUnavailable)
	}

	ctx, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	key, err := c.deps.loadKey(ctx)
	if err == nil && key == "" {
		err = errNotRegistered
	}
	if err != nil {
		if errors.Is(err, errNotRegistered) {
			c.retryAt = now.Add(notRegisteredRetry)
		} else {
			c.retryAt = now.Add(failedRetry)
		}
		log.Printf("AI agent is not ready: %v", err)
		return nil, fmt.Errorf("%w: %v", ai.ErrUnavailable, err)
	}

	limiter, err := c.deps.newLimiter(ctx)
	if err != nil {
		c.retryAt = now.Add(failedRetry)
		log.Printf("AI agent is not ready: %v", err)
		return nil, fmt.Errorf("%w: %v", ai.ErrUnavailable, err)
	}

	c.chat = c.deps.newChat(key, limiter)
	log.Printf("AI agent is ready")

	return c.chat, nil
}
