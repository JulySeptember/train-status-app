// Package dynamostore は、利用者とセッションを DynamoDB に保存する auth.Store（docs/design/login.md 6章）。
//
// テーブルの項目（pk・sk は文字列）:
//
//	USER#<sub>        PROFILE           createdAt
//	USER#<sub>        SESSION#<hash>    expiresAt（退会のときにセッションを引くための目印）
//	SESSION#<hash>    SESSION           userId, expiresAt
//
// expiresAt は TTL（UNIX 秒）で、期限が過ぎると DynamoDB が消す。
//
// 退会と同時に別の端末でログインしても、退会した利用者のセッションや目印が残らないようにする:
//   - セッションと目印は、PROFILE があることを条件に、1つのトランザクションで書く
//   - 期限の延長は、セッションと目印の両方が残っているときだけ行う（消えたものを書き戻さない）
//   - 退会は PROFILE を先に消してから、目印とセッションを引いて消す
package dynamostore

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"train-status-app/backend/internal/auth"
)

// API は、使う DynamoDB の操作。*dynamodb.Client が満たす
type API interface {
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, in *dynamodb.PutItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(ctx context.Context, in *dynamodb.DeleteItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Query(ctx context.Context, in *dynamodb.QueryInput, opts ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	BatchWriteItem(ctx context.Context, in *dynamodb.BatchWriteItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
	TransactWriteItems(ctx context.Context, in *dynamodb.TransactWriteItemsInput, opts ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}

const (
	attrPK        = "pk"
	attrSK        = "sk"
	attrCreatedAt = "createdAt"
	attrUserID    = "userId"
	attrExpires   = "expiresAt"

	skProfile = "PROFILE"
	skSession = "SESSION"

	// BatchWriteItem で1回に送れる件数
	batchSize = 25

	// BatchWriteItem で処理されなかった項目を送り直す回数
	batchRetries = 5
)

type Store struct {
	api   API
	table string
}

func New(api API, table string) *Store {
	return &Store{api: api, table: table}
}

func userPK(userID string) string { return "USER#" + userID }

func sessionPK(hash string) string { return "SESSION#" + hash }

func sessionMarkerSK(hash string) string { return "SESSION#" + hash }

func key(pk, sk string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		attrPK: &types.AttributeValueMemberS{Value: pk},
		attrSK: &types.AttributeValueMemberS{Value: sk},
	}
}

func unix(t time.Time) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatInt(t.Unix(), 10)}
}

func (s *Store) EnsureUser(ctx context.Context, userID string, now time.Time) (auth.User, error) {

	out, err := s.api.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(s.table),
		Key:                       key(userPK(userID), skProfile),
		UpdateExpression:          aws.String("SET #createdAt = if_not_exists(#createdAt, :now)"),
		ExpressionAttributeNames:  map[string]string{"#createdAt": attrCreatedAt},
		ExpressionAttributeValues: map[string]types.AttributeValue{":now": &types.AttributeValueMemberS{Value: now.UTC().Format(time.RFC3339)}},
		ReturnValues:              types.ReturnValueAllNew,
	})
	if err != nil {
		return auth.User{}, err
	}

	return toUser(userID, out.Attributes)
}

func (s *Store) GetUser(ctx context.Context, userID string) (auth.User, error) {

	out, err := s.api.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		Key:            key(userPK(userID), skProfile),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return auth.User{}, err
	}
	if out.Item == nil {
		return auth.User{}, auth.ErrNotFound
	}

	return toUser(userID, out.Item)
}

func toUser(userID string, item map[string]types.AttributeValue) (auth.User, error) {
	v, ok := item[attrCreatedAt].(*types.AttributeValueMemberS)
	if !ok {
		return auth.User{}, fmt.Errorf("user item has no %s", attrCreatedAt)
	}
	createdAt, err := time.Parse(time.RFC3339, v.Value)
	if err != nil {
		return auth.User{}, fmt.Errorf("user item: %w", err)
	}
	return auth.User{ID: userID, CreatedAt: createdAt}, nil
}

// CreateSession は、セッションと利用者の側の目印（退会のときに引く）を書く。
// 利用者がもういなければ（退会したところなら）書かずに ErrNotFound を返す
func (s *Store) CreateSession(ctx context.Context, sess auth.Session) error {

	marker := key(userPK(sess.UserID), sessionMarkerSK(sess.Hash))
	marker[attrExpires] = unix(sess.ExpiresAt)

	item := key(sessionPK(sess.Hash), skSession)
	item[attrUserID] = &types.AttributeValueMemberS{Value: sess.UserID}
	item[attrExpires] = unix(sess.ExpiresAt)

	_, err := s.api.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{ConditionCheck: &types.ConditionCheck{
				TableName:                aws.String(s.table),
				Key:                      key(userPK(sess.UserID), skProfile),
				ConditionExpression:      aws.String("attribute_exists(#pk)"),
				ExpressionAttributeNames: map[string]string{"#pk": attrPK},
			}},
			{Put: &types.Put{TableName: aws.String(s.table), Item: marker}},
			{Put: &types.Put{TableName: aws.String(s.table), Item: item}},
		},
	})
	return transactError(err)
}

// transactError は、トランザクションの条件を満たさなかったことを ErrNotFound にする
func transactError(err error) error {
	var canceled *types.TransactionCanceledException
	if errors.As(err, &canceled) {
		for _, r := range canceled.CancellationReasons {
			if aws.ToString(r.Code) == "ConditionalCheckFailed" {
				return auth.ErrNotFound
			}
		}
	}
	return err
}

func (s *Store) GetSession(ctx context.Context, hash string) (auth.Session, error) {

	out, err := s.api.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		Key:            key(sessionPK(hash), skSession),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return auth.Session{}, err
	}
	if out.Item == nil {
		return auth.Session{}, auth.ErrNotFound
	}

	userID, ok := out.Item[attrUserID].(*types.AttributeValueMemberS)
	if !ok {
		return auth.Session{}, fmt.Errorf("session item has no %s", attrUserID)
	}
	expires, ok := out.Item[attrExpires].(*types.AttributeValueMemberN)
	if !ok {
		return auth.Session{}, fmt.Errorf("session item has no %s", attrExpires)
	}
	sec, err := strconv.ParseInt(expires.Value, 10, 64)
	if err != nil {
		return auth.Session{}, fmt.Errorf("session item: %w", err)
	}

	return auth.Session{Hash: hash, UserID: userID.Value, ExpiresAt: time.Unix(sec, 0)}, nil
}

// ExtendSession は、ログアウトや退会で消えたセッション・目印を書き戻さないよう、両方が残っているときだけ期限を延ばす
func (s *Store) ExtendSession(ctx context.Context, sess auth.Session) error {

	update := func(k map[string]types.AttributeValue) types.TransactWriteItem {
		return types.TransactWriteItem{Update: &types.Update{
			TableName:                 aws.String(s.table),
			Key:                       k,
			UpdateExpression:          aws.String("SET #expires = :expires"),
			ConditionExpression:       aws.String("attribute_exists(#pk)"),
			ExpressionAttributeNames:  map[string]string{"#expires": attrExpires, "#pk": attrPK},
			ExpressionAttributeValues: map[string]types.AttributeValue{":expires": unix(sess.ExpiresAt)},
		}}
	}

	_, err := s.api.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			update(key(sessionPK(sess.Hash), skSession)),
			update(key(userPK(sess.UserID), sessionMarkerSK(sess.Hash))),
		},
	})
	return transactError(err)
}

func (s *Store) DeleteSession(ctx context.Context, sess auth.Session) error {

	for _, k := range []map[string]types.AttributeValue{
		key(sessionPK(sess.Hash), skSession),
		key(userPK(sess.UserID), sessionMarkerSK(sess.Hash)),
	} {
		if _, err := s.api.DeleteItem(ctx, &dynamodb.DeleteItemInput{TableName: aws.String(s.table), Key: k}); err != nil {
			return err
		}
	}
	return nil
}

// DeleteUser は、USER#<sub> の項目をすべて消し、目印から引いたセッションも消す。
// PROFILE を先に消し、このあと新しいセッションが作られないようにしてから、目印を引く
func (s *Store) DeleteUser(ctx context.Context, userID string) error {

	if _, err := s.api.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key:       key(userPK(userID), skProfile),
	}); err != nil {
		return err
	}

	var keys []map[string]types.AttributeValue

	var start map[string]types.AttributeValue
	for {
		out, err := s.api.Query(ctx, &dynamodb.QueryInput{
			TableName:                 aws.String(s.table),
			KeyConditionExpression:    aws.String("#pk = :pk"),
			ProjectionExpression:      aws.String("#pk, #sk"),
			ExpressionAttributeNames:  map[string]string{"#pk": attrPK, "#sk": attrSK},
			ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: userPK(userID)}},
			ExclusiveStartKey:         start,
			ConsistentRead:            aws.Bool(true),
		})
		if err != nil {
			return err
		}

		for _, item := range out.Items {
			sk, ok := item[attrSK].(*types.AttributeValueMemberS)
			if !ok {
				continue
			}
			// セッションを先に消し、目印は後で消す（途中で失敗しても、やり直せば目印からセッションを引ける）
			if hash, ok := strings.CutPrefix(sk.Value, "SESSION#"); ok {
				keys = append([]map[string]types.AttributeValue{key(sessionPK(hash), skSession)}, keys...)
			}
			keys = append(keys, key(userPK(userID), sk.Value))
		}

		if len(out.LastEvaluatedKey) == 0 {
			break
		}
		start = out.LastEvaluatedKey
	}

	for i := 0; i < len(keys); i += batchSize {
		if err := s.batchDelete(ctx, keys[i:min(i+batchSize, len(keys))]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) batchDelete(ctx context.Context, keys []map[string]types.AttributeValue) error {

	requests := make([]types.WriteRequest, len(keys))
	for i, k := range keys {
		requests[i] = types.WriteRequest{DeleteRequest: &types.DeleteRequest{Key: k}}
	}

	for attempt := 0; len(requests) > 0; attempt++ {
		if attempt > batchRetries {
			return fmt.Errorf("batch delete: %d items left unprocessed", len(requests))
		}
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 100 * time.Millisecond):
			}
		}

		out, err := s.api.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{s.table: requests},
		})
		if err != nil {
			return err
		}
		requests = out.UnprocessedItems[s.table]
	}
	return nil
}
