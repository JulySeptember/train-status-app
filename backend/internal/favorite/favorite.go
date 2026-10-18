// Package favorite は、ログインした利用者のお気に入り（駅・路線・経路）を扱う（docs/design/login.md 4章）。
package favorite

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Kind はお気に入りの種類。API のパス（/api/me/favorites/{kind}/{id}）にそのまま使う
type Kind string

const (
	Stations Kind = "stations"
	Railways Kind = "railways"
	Journeys Kind = "journeys"
)

// Limits は、種類ごとに登録できる件数（乱用対策）
var Limits = map[Kind]int{
	Stations: 30,
	Railways: 30,
	Journeys: 20,
}

// journeySeparator は、経路の ID（<出発駅ID>~<到着駅ID>）の区切り。駅 ID には含まれない
const journeySeparator = "~"

var (
	// ErrInvalid は、種類か ID が正しくない（無い駅・路線を含む）こと
	ErrInvalid = errors.New("favorite: invalid kind or id")

	// ErrLimit は、その種類の上限に達していること
	ErrLimit = errors.New("favorite: limit reached")

	// ErrUserNotFound は、利用者がいない（退会したところ）こと
	ErrUserNotFound = errors.New("favorite: user not found")
)

// LimitError は、kind の上限（Limit 件）に達していること。errors.Is(err, ErrLimit) が真になる
type LimitError struct {
	Kind  Kind
	Limit int
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("%v: %s (%d)", ErrLimit, e.Kind, e.Limit)
}

func (e *LimitError) Is(target error) bool { return target == ErrLimit }

// Item は保存するお気に入り
type Item struct {
	Kind      Kind
	ID        string
	CreatedAt time.Time
}

// Store はお気に入りの保存先。利用者を消すと（退会）、お気に入りも消える
type Store interface {
	// ListFavorites は、利用者のお気に入りをすべて返す（順不同）
	ListFavorites(ctx context.Context, userID string) ([]Item, error)

	// PutFavorite は、お気に入りを足す。すでにあれば登録日を変えない。
	// 利用者がいなければ ErrUserNotFound
	PutFavorite(ctx context.Context, userID string, item Item) error

	// DeleteFavorite は、お気に入りを外す。無ければ何もしない
	DeleteFavorite(ctx context.Context, userID string, kind Kind, id string) error
}

// Catalog は、駅・路線があるかを答える（service.Service が満たす）
type Catalog interface {
	HasStation(id string) bool
	HasRailway(id string) bool
}

// JourneyFavorite は、お気に入りの経路（出発駅と到着駅）
type JourneyFavorite struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Favorites は、利用者のお気に入り。種類ごとに登録した順に並べる
type Favorites struct {
	Stations []string          `json:"stations"`
	Railways []string          `json:"railways"`
	Journeys []JourneyFavorite `json:"journeys"`
}

type Service struct {
	store   Store
	catalog Catalog
	now     func() time.Time
}

func NewService(store Store, catalog Catalog) *Service {
	return &Service{store: store, catalog: catalog, now: time.Now}
}

// valid は、種類と ID が正しく、駅・路線が今もあるかを返す
func (s *Service) valid(kind Kind, id string) bool {
	switch kind {
	case Stations:
		return s.catalog.HasStation(id)
	case Railways:
		return s.catalog.HasRailway(id)
	case Journeys:
		from, to, ok := strings.Cut(id, journeySeparator)
		return ok && from != to && s.catalog.HasStation(from) && s.catalog.HasStation(to)
	}
	return false
}

// List は、お気に入りを返す。データの更新で無くなった駅・路線は、消さずに返さないだけにする
func (s *Service) List(ctx context.Context, userID string) (Favorites, error) {

	items, err := s.store.ListFavorites(ctx, userID)
	if err != nil {
		return Favorites{}, err
	}

	slices.SortStableFunc(items, func(a, b Item) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})

	result := Favorites{Stations: []string{}, Railways: []string{}, Journeys: []JourneyFavorite{}}
	for _, item := range items {
		if !s.valid(item.Kind, item.ID) {
			continue
		}
		switch item.Kind {
		case Stations:
			result.Stations = append(result.Stations, item.ID)
		case Railways:
			result.Railways = append(result.Railways, item.ID)
		case Journeys:
			from, to, _ := strings.Cut(item.ID, journeySeparator)
			result.Journeys = append(result.Journeys, JourneyFavorite{From: from, To: to})
		}
	}

	return result, nil
}

// Add は、お気に入りを足す。すでにあれば何もしない
func (s *Service) Add(ctx context.Context, userID string, kind Kind, id string) error {

	if !s.valid(kind, id) {
		return fmt.Errorf("%w: %s %s", ErrInvalid, kind, id)
	}

	items, err := s.store.ListFavorites(ctx, userID)
	if err != nil {
		return err
	}

	count := 0
	for _, item := range items {
		if item.Kind != kind {
			continue
		}
		if item.ID == id {
			return nil
		}
		// データの更新で無くなった駅・路線は、一覧に出ず外せないので数えない
		if s.valid(item.Kind, item.ID) {
			count++
		}
	}
	// 同時に足されたときは少し超えることがあるが、乱用対策としては十分
	if count >= Limits[kind] {
		return &LimitError{Kind: kind, Limit: Limits[kind]}
	}

	return s.store.PutFavorite(ctx, userID, Item{Kind: kind, ID: id, CreatedAt: s.now()})
}

// Remove は、お気に入りを外す。無い駅・路線（データの更新で無くなったもの）も外せる
func (s *Service) Remove(ctx context.Context, userID string, kind Kind, id string) error {
	if _, ok := Limits[kind]; !ok || id == "" {
		return fmt.Errorf("%w: %s %s", ErrInvalid, kind, id)
	}
	return s.store.DeleteFavorite(ctx, userID, kind, id)
}
