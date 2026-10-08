import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ChevronRight } from "lucide-react";

import { api } from "@/api";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import RailwayBadge from "@/components/RailwayBadge";
import PageTitle from "@/components/PageTitle";

import { useRailway } from "@/lib/railways";
import { usePrefetchStation } from "@/lib/station";

export default function Route() {
  const { routeId = "" } = useParams();

  const { data, isPending, error } = useQuery({
    queryKey: ["stations", routeId],
    queryFn: () => api.getStations(routeId),
  });

  const railway = useRailway(routeId);
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

      {/* 駅は路線の順に並んでいるので、路線の色の線でつないで縦に並べる */}
      <ol
        className="max-w-xl border-l-4 pl-4"
        style={{ borderColor: railway?.color }}
      >
        {data.map((station) => (
          <li key={station.id}>
            <Link
              to={`/stations/${encodeURIComponent(station.id)}`}
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
