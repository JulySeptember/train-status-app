import { Link, useParams, useSearchParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ChevronRight } from "lucide-react";

import { api } from "@/api";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import RailwayBadge from "@/components/RailwayBadge";
import PageTitle from "@/components/PageTitle";
import DirectionTabs, { selectDirection } from "@/components/DirectionTabs";

import { useRailway } from "@/lib/railways";
import { stationPath, usePrefetchStation } from "@/lib/station";

export default function Route() {
  const { routeId = "" } = useParams();

  const { data, isPending, error } = useQuery({
    queryKey: ["stations", routeId],
    queryFn: () => api.getStations(routeId),
  });

  const railway = useRailway(routeId);

  // 駅の時刻表は方向ごとなので、駅を選ぶ前に方向も選べるようにする（駅・時刻表の画面と同じ）。
  // 選んだ方向は URL に残し、駅の時刻表から戻ったときも選んだままにする
  const [params, setParams] = useSearchParams();
  const directions = railway?.directions ?? [];
  const direction = selectDirection(directions, params.get("direction"));
  const prefetch = usePrefetchStation();

  if (isPending) return <Loading />;

  if (error) return <Error />;

  return (
    <div className="space-y-6">
      <PageTitle title={railway?.name ?? "駅一覧"} />
      <h1 className="flex items-center gap-3 text-3xl font-bold">
        <RailwayBadge railway={railway} className="size-9 text-base" />
        {railway?.name ?? "駅一覧"}
      </h1>

      <DirectionTabs
        directions={directions}
        value={direction}
        onChange={(value) => setParams({ direction: value }, { replace: true })}
      />

      {/* 駅は路線の順に並んでいるので、路線の色の線でつないで縦に並べる */}
      <ol
        className="max-w-xl border-l-4 pl-4"
        style={{ borderColor: railway?.color }}
      >
        {data.map((station) => (
          <li key={station.id}>
            <Link
              to={stationPath(station.id, direction)}
              {...prefetch(station.id)}
              className="flex items-center justify-between rounded-lg px-3 py-3 transition hover:bg-muted"
            >
              {station.name}
              <ChevronRight size={16} className="text-muted-foreground" />
            </Link>
          </li>
        ))}
      </ol>
    </div>
  );
}
