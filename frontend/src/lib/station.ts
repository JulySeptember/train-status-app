import { useEffect, useRef } from "react";
import { queryOptions, useQueryClient } from "@tanstack/react-query";

import { api } from "@/api";

// 駅の時刻表へのリンク先。方向を選んでいれば、その方向で開く
export function stationPath(id: string, direction?: string) {
  const path = `/stations/${encodeURIComponent(id)}`;
  return direction
    ? `${path}?direction=${encodeURIComponent(direction)}`
    : path;
}

// 駅の詳細（時刻表・乗降人員）。駅の画面と先読みで同じキャッシュを使う
export function stationQuery(id: string) {
  return queryOptions({
    queryKey: ["station", id],
    queryFn: () => api.getStation(id),
    staleTime: 5 * 60 * 1000,
  });
}

// カーソルを載せてから先読みを始めるまでの時間。駅の一覧の上をなぞっただけで、
// 通った駅をすべて取りにいかないようにする（1駅で約110KBある）
const HOVER_DELAY_MS = 150;

// 駅へのリンクをマウスで押し始めたとき・キーボードで選んだとき、またはカーソルを少し載せ続けたときに、
// 駅の詳細を先に取りにいく。本番では API の応答に 0.5〜1 秒かかるので、画面を開くまでの待ち時間を縮める。
// タッチではスクロールで指を置いただけでも pointerdown が起きるので、押し始めでは取りにいかない
export function usePrefetchStation() {
  const queryClient = useQueryClient();
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => () => clearTimeout(timer.current), []);

  return (id: string) => {
    const prefetch = () => {
      clearTimeout(timer.current);
      void queryClient.prefetchQuery(stationQuery(id));
    };

    return {
      // タッチでも指を置くと pointerenter が起きるので、カーソルのときだけ待って取りにいく
      onPointerEnter: (e: React.PointerEvent) => {
        clearTimeout(timer.current);
        if (e.pointerType !== "touch") {
          timer.current = setTimeout(prefetch, HOVER_DELAY_MS);
        }
      },
      onPointerLeave: () => clearTimeout(timer.current),
      onPointerDown: (e: React.PointerEvent) => {
        if (e.pointerType !== "touch") {
          prefetch();
        }
      },
      onFocus: prefetch,
    };
  };
}
