package dynamostore

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"train-status-app/backend/internal/auth"
)

// fakeAPI は、このパッケージが使う式だけを解釈するメモリ上の DynamoDB
type fakeAPI struct {
	items map[string]map[string]types.AttributeValue

	// BatchWriteItem で、最初の1回だけ処理しない件数
	unprocessedOnce int
	batchCalls      int
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{items: map[string]map[string]types.AttributeValue{}}
}

func s(v types.AttributeValue) string {
	switch v := v.(type) {
	case *types.AttributeValueMemberS:
		return v.Value
	case *types.AttributeValueMemberN:
		return v.Value
	}
	return ""
}

func id(k map[string]types.AttributeValue) string { return s(k[attrPK]) + "|" + s(k[attrSK]) }

func (f *fakeAPI) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return &dynamodb.GetItemOutput{Item: f.items[id(in.Key)]}, nil
}

func (f *fakeAPI) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	f.items[id(in.Item)] = maps.Clone(in.Item)
	return &dynamodb.PutItemOutput{}, nil
}

func (f *fakeAPI) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	k := id(in.Key)
	item, ok := f.items[k]
	if in.ConditionExpression != nil && !ok {
		return nil, &types.ConditionalCheckFailedException{}
	}
	if !ok {
		item = maps.Clone(in.Key)
	}
	switch {
	case strings.Contains(*in.UpdateExpression, "if_not_exists"):
		if _, exists := item[attrCreatedAt]; !exists {
			item[attrCreatedAt] = in.ExpressionAttributeValues[":now"]
		}
	default:
		item[attrExpires] = in.ExpressionAttributeValues[":expires"]
	}
	f.items[k] = item
	return &dynamodb.UpdateItemOutput{Attributes: maps.Clone(item)}, nil
}

func (f *fakeAPI) DeleteItem(_ context.Context, in *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	delete(f.items, id(in.Key))
	return &dynamodb.DeleteItemOutput{}, nil
}

// Query は1件ずつページを分けて返し、ページ送りも確かめる
func (f *fakeAPI) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	pk := s(in.ExpressionAttributeValues[":pk"])
	var ids []string
	for k := range f.items {
		if strings.HasPrefix(k, pk+"|") {
			ids = append(ids, k)
		}
	}
	slices.Sort(ids)

	startAfter := ""
	if in.ExclusiveStartKey != nil {
		startAfter = id(in.ExclusiveStartKey)
	}
	for _, k := range ids {
		if k <= startAfter {
			continue
		}
		item := f.items[k]
		page := map[string]types.AttributeValue{attrPK: item[attrPK], attrSK: item[attrSK]}
		return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{page}, LastEvaluatedKey: page}, nil
	}
	return &dynamodb.QueryOutput{}, nil
}

func (f *fakeAPI) BatchWriteItem(_ context.Context, in *dynamodb.BatchWriteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
	f.batchCalls++
	out := &dynamodb.BatchWriteItemOutput{UnprocessedItems: map[string][]types.WriteRequest{}}
	for table, reqs := range in.RequestItems {
		if len(reqs) > batchSize {
			return nil, errors.New("too many items in a batch")
		}
		for i, r := range reqs {
			if i < f.unprocessedOnce {
				out.UnprocessedItems[table] = append(out.UnprocessedItems[table], r)
				continue
			}
			delete(f.items, id(r.DeleteRequest.Key))
		}
	}
	f.unprocessedOnce = 0
	return out, nil
}

func TestStore(t *testing.T) {

	ctx := context.Background()
	api := newFakeAPI()
	store := New(api, "users")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	user, err := store.EnsureUser(ctx, "u1", now)
	if err != nil || !user.CreatedAt.Equal(now) {
		t.Fatalf("unexpected user %+v %v", user, err)
	}
	if user, err := store.EnsureUser(ctx, "u1", now.Add(time.Hour)); err != nil || !user.CreatedAt.Equal(now) {
		t.Fatalf("created at should not change: %+v %v", user, err)
	}
	if _, err := store.GetUser(ctx, "nobody"); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	sess := auth.Session{Hash: "h1", UserID: "u1", ExpiresAt: now.Add(auth.SessionTTL)}
	if err := store.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetSession(ctx, "h1")
	if err != nil || got.Hash != "h1" || got.UserID != "u1" || !got.ExpiresAt.Equal(sess.ExpiresAt) {
		t.Fatalf("unexpected session %+v %v", got, err)
	}
	if api.items["USER#u1|SESSION#h1"] == nil {
		t.Fatal("session marker should be written")
	}

	sess.ExpiresAt = sess.ExpiresAt.Add(24 * time.Hour)
	if err := store.ExtendSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.GetSession(ctx, "h1"); !got.ExpiresAt.Equal(sess.ExpiresAt) {
		t.Fatalf("not extended: %+v", got)
	}
	if s(api.items["USER#u1|SESSION#h1"][attrExpires]) != s(unix(sess.ExpiresAt)) {
		t.Fatal("marker should be extended too")
	}

	if err := store.DeleteSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSession(ctx, "h1"); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if api.items["USER#u1|SESSION#h1"] != nil {
		t.Fatal("marker should be deleted")
	}

	// 消えたセッションは延ばさない（作り直さない）
	if err := store.ExtendSession(ctx, sess); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if api.items["SESSION#h1|SESSION"] != nil {
		t.Fatal("deleted session must not be recreated")
	}
}

func TestDeleteUser(t *testing.T) {

	ctx := context.Background()
	api := newFakeAPI()
	store := New(api, "users")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	if _, err := store.EnsureUser(ctx, "u1", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnsureUser(ctx, "u2", now); err != nil {
		t.Fatal(err)
	}

	// 25件を超えて、BatchWriteItem が複数回になるようにする
	for i := range 20 {
		h := "h" + string(rune('a'+i))
		if err := store.CreateSession(ctx, auth.Session{Hash: h, UserID: "u1", ExpiresAt: now.Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.CreateSession(ctx, auth.Session{Hash: "other", UserID: "u2", ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	api.unprocessedOnce = 3

	if err := store.DeleteUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}

	var left []string
	for k := range api.items {
		left = append(left, k)
	}
	slices.Sort(left)
	want := []string{"SESSION#other|SESSION", "USER#u2|PROFILE", "USER#u2|SESSION#other"}
	if !slices.Equal(left, want) {
		t.Fatalf("left items %v, want %v", left, want)
	}
	if api.batchCalls < 3 {
		t.Fatalf("expected multiple batches with a retry, got %d calls", api.batchCalls)
	}
}

// 実際の *dynamodb.Client が API を満たすこと
var _ API = (*dynamodb.Client)(nil)
