package dynamostore

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"train-status-app/backend/internal/auth"
	"train-status-app/backend/internal/favorite"
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
	prefix := s(in.ExpressionAttributeValues[":prefix"])
	var ids []string
	for k := range f.items {
		if strings.HasPrefix(k, pk+"|"+prefix) {
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
		last := map[string]types.AttributeValue{attrPK: item[attrPK], attrSK: item[attrSK]}
		page := maps.Clone(item)
		if in.ProjectionExpression != nil {
			page = last
		}
		return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{page}, LastEvaluatedKey: last}, nil
	}
	return &dynamodb.QueryOutput{}, nil
}

// TransactWriteItems は、条件（attribute_exists）をすべて確かめてから書く
func (f *fakeAPI) TransactWriteItems(_ context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	reasons := make([]types.CancellationReason, len(in.TransactItems))
	failed := false
	for i, t := range in.TransactItems {
		var k map[string]types.AttributeValue
		switch {
		case t.ConditionCheck != nil:
			k = t.ConditionCheck.Key
		case t.Update != nil:
			k = t.Update.Key
		default:
			continue
		}
		// 条件の無い Update（お気に入りの登録日）は、項目が無くても作る
		if t.Update != nil && t.Update.ConditionExpression == nil {
			continue
		}
		if f.items[id(k)] == nil {
			reasons[i].Code = aws.String("ConditionalCheckFailed")
			failed = true
		}
	}
	if failed {
		return nil, &types.TransactionCanceledException{CancellationReasons: reasons}
	}
	for _, t := range in.TransactItems {
		switch {
		case t.Put != nil:
			f.items[id(t.Put.Item)] = maps.Clone(t.Put.Item)
		case t.Update != nil && strings.Contains(*t.Update.UpdateExpression, "if_not_exists"):
			k := id(t.Update.Key)
			if f.items[k] == nil {
				f.items[k] = maps.Clone(t.Update.Key)
			}
			if _, ok := f.items[k][attrCreatedAt]; !ok {
				f.items[k][attrCreatedAt] = t.Update.ExpressionAttributeValues[":now"]
			}
		case t.Update != nil:
			f.items[id(t.Update.Key)][attrExpires] = t.Update.ExpressionAttributeValues[":expires"]
		}
	}
	return &dynamodb.TransactWriteItemsOutput{}, nil
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
	if api.items["SESSION#h1|SESSION"] != nil || api.items["USER#u1|SESSION#h1"] != nil {
		t.Fatal("deleted session must not be recreated")
	}
}

// 退会と同時に別の端末でログイン・延長しても、退会した利用者の項目が残らない
func TestStoreAfterDeleteUser(t *testing.T) {

	ctx := context.Background()
	api := newFakeAPI()
	store := New(api, "users")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	if _, err := store.EnsureUser(ctx, "u1", now); err != nil {
		t.Fatal(err)
	}
	sess := auth.Session{Hash: "h1", UserID: "u1", ExpiresAt: now.Add(time.Hour)}
	if err := store.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	// 延長の途中で目印だけが消えていても、セッションも目印も書き戻さない
	delete(api.items, "USER#u1|SESSION#h1")
	sess.ExpiresAt = now.Add(2 * time.Hour)
	if err := store.ExtendSession(ctx, sess); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if api.items["USER#u1|SESSION#h1"] != nil {
		t.Fatal("marker must not be recreated")
	}

	if err := store.DeleteUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}

	// 退会したあとは、セッションを作れない
	if err := store.CreateSession(ctx, auth.Session{Hash: "h2", UserID: "u1", ExpiresAt: now.Add(time.Hour)}); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	// 上で手で目印だけを消したセッションは引けないので残る（実際には、目印はセッションのあとに消す）
	for k := range api.items {
		if k != "SESSION#h1|SESSION" {
			t.Errorf("item left after delete: %s", k)
		}
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

func TestFavorites(t *testing.T) {

	ctx := context.Background()
	api := newFakeAPI()
	store := New(api, "users")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	// 利用者がいなければ足せない
	if err := store.PutFavorite(ctx, "u1", favorite.Item{Kind: favorite.Stations, ID: "s1", CreatedAt: now}); !errors.Is(err, favorite.ErrUserNotFound) {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}

	if _, err := store.EnsureUser(ctx, "u1", now); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(ctx, auth.Session{Hash: "h1", UserID: "u1", ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	journey := "odpt.Station:Toei.Asakusa.Asakusa~odpt.Station:Toei.Oedo.Shinjuku"
	for i, item := range []favorite.Item{
		{Kind: favorite.Stations, ID: "odpt.Station:Toei.Oedo.Shinjuku"},
		{Kind: favorite.Railways, ID: "odpt.Railway:Toei.Asakusa"},
		{Kind: favorite.Journeys, ID: journey},
	} {
		item.CreatedAt = now.Add(time.Duration(i) * time.Minute)
		if err := store.PutFavorite(ctx, "u1", item); err != nil {
			t.Fatal(err)
		}
	}

	// もう一度足しても登録日は変わらない
	if err := store.PutFavorite(ctx, "u1", favorite.Item{Kind: favorite.Journeys, ID: journey, CreatedAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	items, err := store.ListFavorites(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("want 3 favorites (sessions excluded), got %+v", items)
	}
	for _, item := range items {
		if item.Kind == favorite.Journeys && (item.ID != journey || !item.CreatedAt.Equal(now.Add(2*time.Minute))) {
			t.Fatalf("unexpected journey %+v", item)
		}
	}

	if err := store.DeleteFavorite(ctx, "u1", favorite.Railways, "odpt.Railway:Toei.Asakusa"); err != nil {
		t.Fatal(err)
	}
	if items, _ := store.ListFavorites(ctx, "u1"); len(items) != 2 {
		t.Fatalf("want 2 favorites after delete, got %+v", items)
	}

	// 退会でお気に入りも消える
	if err := store.DeleteUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if len(api.items) != 0 {
		t.Fatalf("items left after delete: %v", slices.Collect(maps.Keys(api.items)))
	}
}
