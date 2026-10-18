package dynamostore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"train-status-app/backend/internal/auth"
	"train-status-app/backend/internal/favorite"
)

// お気に入りの項目（docs/design/login.md 6章）:
//
//	USER#<sub>    FAV#<種類>#<ID>    createdAt
//
// 利用者の項目なので、退会（DeleteUser）でいっしょに消える。

const favoritePrefix = "FAV#"

func favoriteSK(kind favorite.Kind, id string) string {
	return favoritePrefix + string(kind) + "#" + id
}

func (s *Store) ListFavorites(ctx context.Context, userID string) ([]favorite.Item, error) {

	var items []favorite.Item

	var start map[string]types.AttributeValue
	for {
		out, err := s.api.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			KeyConditionExpression: aws.String("#pk = :pk AND begins_with(#sk, :prefix)"),
			ExpressionAttributeNames: map[string]string{
				"#pk": attrPK,
				"#sk": attrSK,
			},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk":     &types.AttributeValueMemberS{Value: userPK(userID)},
				":prefix": &types.AttributeValueMemberS{Value: favoritePrefix},
			},
			ExclusiveStartKey: start,
			ConsistentRead:    aws.Bool(true),
		})
		if err != nil {
			return nil, err
		}

		for _, item := range out.Items {
			fav, err := toFavorite(item)
			if err != nil {
				return nil, err
			}
			items = append(items, fav)
		}

		if len(out.LastEvaluatedKey) == 0 {
			return items, nil
		}
		start = out.LastEvaluatedKey
	}
}

func toFavorite(item map[string]types.AttributeValue) (favorite.Item, error) {

	sk, _ := item[attrSK].(*types.AttributeValueMemberS)
	if sk == nil {
		return favorite.Item{}, fmt.Errorf("favorite item has no %s", attrSK)
	}
	kind, id, ok := strings.Cut(strings.TrimPrefix(sk.Value, favoritePrefix), "#")
	if !ok {
		return favorite.Item{}, fmt.Errorf("unexpected favorite key %q", sk.Value)
	}

	var createdAt time.Time
	if v, ok := item[attrCreatedAt].(*types.AttributeValueMemberS); ok {
		t, err := time.Parse(time.RFC3339Nano, v.Value)
		if err != nil {
			return favorite.Item{}, fmt.Errorf("favorite item: %w", err)
		}
		createdAt = t
	}

	return favorite.Item{Kind: favorite.Kind(kind), ID: id, CreatedAt: createdAt}, nil
}

// PutFavorite は、退会と同時に足されても残らないよう、PROFILE があることを条件に書く
func (s *Store) PutFavorite(ctx context.Context, userID string, item favorite.Item) error {

	_, err := s.api.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{ConditionCheck: &types.ConditionCheck{
				TableName:                aws.String(s.table),
				Key:                      key(userPK(userID), skProfile),
				ConditionExpression:      aws.String("attribute_exists(#pk)"),
				ExpressionAttributeNames: map[string]string{"#pk": attrPK},
			}},
			{Update: &types.Update{
				TableName:                aws.String(s.table),
				Key:                      key(userPK(userID), favoriteSK(item.Kind, item.ID)),
				UpdateExpression:         aws.String("SET #createdAt = if_not_exists(#createdAt, :now)"),
				ExpressionAttributeNames: map[string]string{"#createdAt": attrCreatedAt},
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":now": &types.AttributeValueMemberS{Value: item.CreatedAt.UTC().Format(time.RFC3339Nano)},
				},
			}},
		},
	})

	err = transactError(err)
	if errors.Is(err, auth.ErrNotFound) {
		return favorite.ErrUserNotFound
	}
	return err
}

func (s *Store) DeleteFavorite(ctx context.Context, userID string, kind favorite.Kind, id string) error {
	_, err := s.api.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key:       key(userPK(userID), favoriteSK(kind, id)),
	})
	return err
}
