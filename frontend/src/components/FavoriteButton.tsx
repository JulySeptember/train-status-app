import { Link } from "react-router-dom";
import { Star } from "lucide-react";

import { Button } from "@/components/ui/button";

import { useAuthConfig, useMe } from "@/lib/auth";
import { useToggleFavorite } from "@/lib/favorites";
import { cn } from "@/lib/utils";
import type { FavoriteKind } from "@/types";

// お気に入りに登録・解除する星のボタン。ログインしていなければ、押すとアカウントの画面を開く。
// label を渡すと、星の横に文言を出す（経路検索の結果など、星だけでは何を登録するか分かりにくいところ）
export default function FavoriteButton({
  kind,
  id,
  name,
  label,
}: {
  kind: FavoriteKind;
  id: string;
  // 読み上げ用の名前（例: 新宿駅）
  name: string;
  label?: boolean;
}) {
  const config = useAuthConfig();
  const me = useMe();
  const favorite = useToggleFavorite(kind, id);

  // ログインを使えないときは出さない
  if (!config.data) {
    return null;
  }

  if (!me.data) {
    return (
      <Button
        variant="ghost"
        size={label ? "sm" : "icon"}
        title="ログインすると、お気に入りに登録できます"
        aria-label={`${name}をお気に入りに登録（ログインが必要です）`}
        render={<Link to="/account" />}
        nativeButton={false}
        className="text-muted-foreground"
      >
        <Star />
        {label && "お気に入りに登録"}
      </Button>
    );
  }

  const text = favorite.active ? "お気に入りから外す" : "お気に入りに登録";

  return (
    <span className="inline-flex flex-col items-start gap-1">
      <Button
        variant="ghost"
        size={label ? "sm" : "icon"}
        title={text}
        aria-label={`${name}を${text}`}
        aria-pressed={favorite.active}
        disabled={!favorite.ready || favorite.pending}
        onClick={favorite.toggle}
        className={cn(
          favorite.active ? "text-warning" : "text-muted-foreground",
        )}
      >
        <Star className={cn(favorite.active && "fill-current")} />
        {label && text}
      </Button>

      {favorite.error && (
        <span role="alert" className="text-xs text-destructive-foreground">
          {favorite.error.message}
        </span>
      )}
    </span>
  );
}
