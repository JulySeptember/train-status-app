// Package dynamolimit は、利用上限（設計書 9章）を DynamoDB で数える ai.Limiter。
// Lambda はインスタンスごとにメモリが分かれるので、回数は DynamoDB の1つのテーブルで数える。
package dynamolimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"train-status-app/backend/internal/ai"
)

// API は、使う DynamoDB の操作。*dynamodb.Client が満たす
type API interface {
	UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

// テーブルの項目: pk（文字列）、count（数）、expiresAt（TTL、UNIX 秒）
const (
	attrKey     = "pk"
	attrCount   = "count"
	attrExpires = "expiresAt"
)

type Limiter struct {
	api    API
	table  string
	limits ai.Limits
	now    func() time.Time
}

func New(api API, table string, limits ai.Limits) *Limiter {
	return &Limiter{api: api, table: table, limits: limits, now: time.Now}
}

// ipKey は IP をそのまま保存しないためのハッシュ
func ipKey(ip string) string {
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:8])
}

func (l *Limiter) AllowQuestion(ctx context.Context, clientIP string) error {
	now := l.now()
	ip := ipKey(clientIP)

	minute := fmt.Sprintf("ip#%s#minute#%d", ip, now.Unix()/60)
	if err := l.increment(ctx, minute, l.limits.PerIPPerMinute, now.Add(2*time.Minute)); err != nil {
		return err
	}

	day := fmt.Sprintf("ip#%s#day#%s", ip, ai.QuotaDay(now))
	return l.increment(ctx, day, l.limits.PerIPPerDay, now.Add(48*time.Hour))
}

func (l *Limiter) AllowCall(ctx context.Context) error {
	now := l.now()
	return l.increment(ctx, "app#day#"+ai.QuotaDay(now), l.limits.CallsPerDay, now.Add(48*time.Hour))
}

// increment は key の回数を1つ増やす。すでに limit 回に達していれば増やさずに ErrQuotaExceeded を返す。
// DynamoDB に書けないときは、共有の無料枠を守るため、使えないものとして扱う（ErrUnavailable）。
func (l *Limiter) increment(ctx context.Context, key string, limit int, expires time.Time) error {

	_, err := l.api.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(l.table),
		Key: map[string]types.AttributeValue{
			attrKey: &types.AttributeValueMemberS{Value: key},
		},
		UpdateExpression:    aws.String("ADD #count :one SET #expires = if_not_exists(#expires, :expires)"),
		ConditionExpression: aws.String("attribute_not_exists(#count) OR #count < :limit"),
		ExpressionAttributeNames: map[string]string{
			"#count":   attrCount,
			"#expires": attrExpires,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one":     &types.AttributeValueMemberN{Value: "1"},
			":limit":   &types.AttributeValueMemberN{Value: strconv.Itoa(limit)},
			":expires": &types.AttributeValueMemberN{Value: strconv.FormatInt(expires.Unix(), 10)},
		},
	})

	var failed *types.ConditionalCheckFailedException
	switch {
	case errors.As(err, &failed):
		return fmt.Errorf("%w: %s", ai.ErrQuotaExceeded, keyKind(key))
	case err != nil:
		return fmt.Errorf("%w: usage table: %v", ai.ErrUnavailable, err)
	}
	return nil
}

// keyKind は、ログに出す上限の種類（IP のハッシュは出さない）
func keyKind(key string) string {
	switch {
	case strings.HasPrefix(key, "app#"):
		return "app daily calls"
	case strings.Contains(key, "#minute#"):
		return "per-ip per-minute"
	}
	return "per-ip per-day"
}
