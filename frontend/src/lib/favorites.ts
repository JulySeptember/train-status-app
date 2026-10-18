import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api } from "@/api";
import type { FavoriteKind, Favorites } from "@/types";

import { useMe } from "@/lib/auth";

// lib/auth.ts もログアウトのときにこのキーを消す
const FAVORITES_KEY = ["favorites"];

// 経路のお気に入りの id（バックエンドと同じ「出発駅ID~到着駅ID」）
export function journeyFavoriteId(from: string, to: string) {
  return `${from}~${to}`;
}

function ids(favorites: Favorites, kind: FavoriteKind): string[] {
  return kind === "journeys"
    ? favorites.journeys.map((j) => journeyFavoriteId(j.from, j.to))
    : favorites[kind];
}

function withItem(
  favorites: Favorites,
  kind: FavoriteKind,
  id: string,
  add: boolean,
): Favorites {
  if (kind === "journeys") {
    const rest = favorites.journeys.filter(
      (j) => journeyFavoriteId(j.from, j.to) !== id,
    );
    const [from, to] = id.split("~");
    return {
      ...favorites,
      journeys: add ? [...rest, { from, to }] : rest,
    };
  }

  const rest = favorites[kind].filter((v) => v !== id);
  return { ...favorites, [kind]: add ? [...rest, id] : rest };
}

// ログイン中の利用者のお気に入り。ログインしていなければ取りに行かず、data は undefined
export function useFavorites() {
  const me = useMe();

  return useQuery({
    queryKey: FAVORITES_KEY,
    queryFn: api.getFavorites,
    enabled: !!me.data,
  });
}

// お気に入りの登録・解除。画面にはすぐ反映し、失敗したら元に戻す
export function useToggleFavorite(kind: FavoriteKind, id: string) {
  const client = useQueryClient();
  const favorites = useFavorites();

  const active = favorites.data
    ? ids(favorites.data, kind).includes(id)
    : false;

  const mutation = useMutation({
    mutationFn: (add: boolean) =>
      add ? api.addFavorite(kind, id) : api.removeFavorite(kind, id),
    onMutate: async (add) => {
      await client.cancelQueries({ queryKey: FAVORITES_KEY });
      const previous = client.getQueryData<Favorites>(FAVORITES_KEY);
      if (previous) {
        client.setQueryData(FAVORITES_KEY, withItem(previous, kind, id, add));
      }
      return { previous };
    },
    onError: (_err, _add, context) => {
      if (context?.previous) {
        client.setQueryData(FAVORITES_KEY, context.previous);
      }
    },
    onSettled: () => client.invalidateQueries({ queryKey: FAVORITES_KEY }),
  });

  return {
    // お気に入りを読み込めていなければ、登録・解除できない
    ready: favorites.isSuccess,
    active,
    toggle: () => mutation.mutate(!active),
    pending: mutation.isPending,
    error: mutation.error,
  };
}
