package dynamostore

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

	"train-status-app/backend/internal/auth"
	"train-status-app/backend/internal/notify"
)

// 通知の項目（docs/design/login.md 6章）:
//
//	USER#<sub>    ALERT#<路線ID>             days, start, end, gsi1pk = ALERT#<路線ID>, gsi1sk = USER#<sub>
//	USER#<sub>    PUSH#<endpoint の SHA-256>  endpoint, p256dh, auth, createdAt
//	USER#<sub>    PUSHTEST                  testedAt, expiresAt（試しの通知の間隔。TTL で消える）
//	NOTIFY        STATE                     levels（路線ID → 状態）
//
// 利用者の項目なので、退会（DeleteUser）でいっしょに消える。GSI（gsi1）で路線を見ている利用者を引く。

const (
	alertPrefix = "ALERT#"
	pushPrefix  = "PUSH#"
	skPushTest  = "PUSHTEST"

	pkNotify = "NOTIFY"
	skState  = "STATE"

	// 路線を見ている利用者を引く GSI（infra/main/dynamodb.tf）
	gsiAlerts = "gsi1"

	attrGSIPK     = "gsi1pk"
	attrGSISK     = "gsi1sk"
	attrDays      = "days"
	attrStart     = "start"
	attrEnd       = "end"
	attrEndpoint  = "endpoint"
	attrP256dh    = "p256dh"
	attrAuth      = "auth"
	attrTestedAt  = "testedAt"
	attrLevels    = "levels"
	attrRailwayID = "railwayId"
)

func pushSK(endpoint string) string {
	sum := sha256.Sum256([]byte(endpoint))
	return pushPrefix + hex.EncodeToString(sum[:])
}

func str(item map[string]types.AttributeValue, name string) string {
	if v, ok := item[name].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

// putUserItem は、退会と同時に書かれても残らないよう、PROFILE があることを条件に書く
func (s *Store) putUserItem(ctx context.Context, userID string, item map[string]types.AttributeValue) error {
	_, err := s.api.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{ConditionCheck: &types.ConditionCheck{
				TableName:                aws.String(s.table),
				Key:                      key(userPK(userID), skProfile),
				ConditionExpression:      aws.String("attribute_exists(#pk)"),
				ExpressionAttributeNames: map[string]string{"#pk": attrPK},
			}},
			{Put: &types.Put{TableName: aws.String(s.table), Item: item}},
		},
	})
	return transactError(err)
}

// queryUser は、利用者の sk が prefix で始まる項目をすべて返す
func (s *Store) queryUser(ctx context.Context, userID, prefix string) ([]map[string]types.AttributeValue, error) {

	var items []map[string]types.AttributeValue
	var start map[string]types.AttributeValue
	for {
		out, err := s.api.Query(ctx, &dynamodb.QueryInput{
			TableName:                aws.String(s.table),
			KeyConditionExpression:   aws.String("#pk = :pk AND begins_with(#sk, :prefix)"),
			ExpressionAttributeNames: map[string]string{"#pk": attrPK, "#sk": attrSK},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk":     &types.AttributeValueMemberS{Value: userPK(userID)},
				":prefix": &types.AttributeValueMemberS{Value: prefix},
			},
			ExclusiveStartKey: start,
			ConsistentRead:    aws.Bool(true),
		})
		if err != nil {
			return nil, err
		}
		items = append(items, out.Items...)
		if len(out.LastEvaluatedKey) == 0 {
			return items, nil
		}
		start = out.LastEvaluatedKey
	}
}

func toAlert(item map[string]types.AttributeValue) notify.Alert {
	a := notify.Alert{
		RailwayID: str(item, attrRailwayID),
		Start:     str(item, attrStart),
		End:       str(item, attrEnd),
	}
	if days, ok := item[attrDays].(*types.AttributeValueMemberSS); ok {
		for _, d := range days.Value {
			a.Days = append(a.Days, notify.Day(d))
		}
	}
	return a
}

func (s *Store) ListAlerts(ctx context.Context, userID string) ([]notify.Alert, error) {
	items, err := s.queryUser(ctx, userID, alertPrefix)
	if err != nil {
		return nil, err
	}
	alerts := make([]notify.Alert, 0, len(items))
	for _, item := range items {
		alerts = append(alerts, toAlert(item))
	}
	return alerts, nil
}

func (s *Store) PutAlert(ctx context.Context, userID string, alert notify.Alert) error {

	days := make([]string, len(alert.Days))
	for i, d := range alert.Days {
		days[i] = string(d)
	}

	item := key(userPK(userID), alertPrefix+alert.RailwayID)
	item[attrRailwayID] = &types.AttributeValueMemberS{Value: alert.RailwayID}
	item[attrDays] = &types.AttributeValueMemberSS{Value: days}
	item[attrStart] = &types.AttributeValueMemberS{Value: alert.Start}
	item[attrEnd] = &types.AttributeValueMemberS{Value: alert.End}
	item[attrGSIPK] = &types.AttributeValueMemberS{Value: alertPrefix + alert.RailwayID}
	item[attrGSISK] = &types.AttributeValueMemberS{Value: userPK(userID)}

	err := s.putUserItem(ctx, userID, item)
	if errors.Is(err, auth.ErrNotFound) {
		return notify.ErrUserNotFound
	}
	return err
}

func (s *Store) DeleteAlert(ctx context.Context, userID, railwayID string) error {
	_, err := s.api.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key:       key(userPK(userID), alertPrefix+railwayID),
	})
	return err
}

func (s *Store) AlertsForRailway(ctx context.Context, railwayID string) ([]notify.UserAlert, error) {

	var result []notify.UserAlert
	var start map[string]types.AttributeValue
	for {
		out, err := s.api.Query(ctx, &dynamodb.QueryInput{
			TableName:                aws.String(s.table),
			IndexName:                aws.String(gsiAlerts),
			KeyConditionExpression:   aws.String("#gpk = :gpk"),
			ExpressionAttributeNames: map[string]string{"#gpk": attrGSIPK},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":gpk": &types.AttributeValueMemberS{Value: alertPrefix + railwayID},
			},
			ExclusiveStartKey: start,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range out.Items {
			userID, ok := strings.CutPrefix(str(item, attrPK), "USER#")
			if !ok {
				continue
			}
			result = append(result, notify.UserAlert{UserID: userID, Alert: toAlert(item)})
		}
		if len(out.LastEvaluatedKey) == 0 {
			return result, nil
		}
		start = out.LastEvaluatedKey
	}
}

func (s *Store) ListSubscriptions(ctx context.Context, userID string) ([]notify.Subscription, error) {

	items, err := s.queryUser(ctx, userID, pushPrefix)
	if err != nil {
		return nil, err
	}

	subs := make([]notify.Subscription, 0, len(items))
	for _, item := range items {
		var sub notify.Subscription
		sub.Endpoint = str(item, attrEndpoint)
		sub.Keys.P256dh = str(item, attrP256dh)
		sub.Keys.Auth = str(item, attrAuth)
		if t, err := time.Parse(time.RFC3339Nano, str(item, attrCreatedAt)); err == nil {
			sub.CreatedAt = t
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

func (s *Store) PutSubscription(ctx context.Context, userID string, sub notify.Subscription) error {

	item := key(userPK(userID), pushSK(sub.Endpoint))
	item[attrEndpoint] = &types.AttributeValueMemberS{Value: sub.Endpoint}
	item[attrP256dh] = &types.AttributeValueMemberS{Value: sub.Keys.P256dh}
	item[attrAuth] = &types.AttributeValueMemberS{Value: sub.Keys.Auth}
	item[attrCreatedAt] = &types.AttributeValueMemberS{Value: sub.CreatedAt.UTC().Format(time.RFC3339Nano)}

	err := s.putUserItem(ctx, userID, item)
	if errors.Is(err, auth.ErrNotFound) {
		return notify.ErrUserNotFound
	}
	return err
}

func (s *Store) DeleteSubscription(ctx context.Context, userID, endpoint string) error {
	_, err := s.api.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key:       key(userPK(userID), pushSK(endpoint)),
	})
	return err
}

// TakeTestSlot は、前に送った時刻が interval より前のときだけ書く（条件付きの PutItem）
func (s *Store) TakeTestSlot(ctx context.Context, userID string, now time.Time, interval time.Duration) (bool, error) {

	item := key(userPK(userID), skPushTest)
	item[attrTestedAt] = &types.AttributeValueMemberN{Value: strconv.FormatInt(now.Unix(), 10)}
	item[attrExpires] = unix(now.Add(interval + time.Hour))

	_, err := s.api.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:                aws.String(s.table),
		Item:                     item,
		ConditionExpression:      aws.String("attribute_not_exists(#pk) OR #testedAt <= :cutoff"),
		ExpressionAttributeNames: map[string]string{"#pk": attrPK, "#testedAt": attrTestedAt},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":cutoff": &types.AttributeValueMemberN{Value: strconv.FormatInt(now.Add(-interval).Unix(), 10)},
		},
	})
	var failed *types.ConditionalCheckFailedException
	if errors.As(err, &failed) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) LoadStates(ctx context.Context) (map[string]notify.Level, error) {

	out, err := s.api.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		Key:            key(pkNotify, skState),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}

	result := map[string]notify.Level{}
	levels, ok := out.Item[attrLevels].(*types.AttributeValueMemberM)
	if !ok {
		return result, nil
	}
	for id, v := range levels.Value {
		if s, ok := v.(*types.AttributeValueMemberS); ok {
			result[id] = notify.Level(s.Value)
		}
	}
	return result, nil
}

func (s *Store) SaveStates(ctx context.Context, states map[string]notify.Level) error {

	levels := make(map[string]types.AttributeValue, len(states))
	for id, l := range states {
		levels[id] = &types.AttributeValueMemberS{Value: string(l)}
	}

	item := key(pkNotify, skState)
	item[attrLevels] = &types.AttributeValueMemberM{Value: levels}

	if _, err := s.api.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(s.table), Item: item}); err != nil {
		return fmt.Errorf("save states: %w", err)
	}
	return nil
}
