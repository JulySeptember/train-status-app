import { queryOptions, useQueryClient } from "@tanstack/react-query";

import { api } from "@/api";

// 駅の詳細（時刻表・乗降人員）。駅の画面と先読みで同じキャッシュを使う
export function stationQuery(id: string) {
  return queryOptions({
    queryKey: ["station", id],
    queryFn: () => api.getStation(id),
    staleTime: 5 * 60 * 1000,
  });
}

// 駅へのリンクにカーソルを載せたとき・押し始めたときに、駅の詳細を先に取りにいく。
// 本番では API の応答に 0.5〜1 秒かかるので、画面を開くまでの待ち時間を縮める
export function usePrefetchStation() {
  const queryClient = useQueryClient();

  return (id: string) => {
    const prefetch = () => void queryClient.prefetchQuery(stationQuery(id));

    return {
      onPointerEnter: prefetch,
      onPointerDown: prefetch,
      onFocus: prefetch,
    };
  };
}
