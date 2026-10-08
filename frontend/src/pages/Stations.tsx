import { useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ChevronRight, Search } from "lucide-react";

import { api } from "@/api";
import { type Railway, type Station } from "@/types";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import RailwayBadge from "@/components/RailwayBadge";
import PageTitle from "@/components/PageTitle";

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Input } from "@/components/ui/input";
import { railwayIdOf, useRailways } from "@/lib/railways";

type Group = {
  railway: Railway;
  stations: Station[];
};

function StationGrid({ stations }: { stations: Station[] }) {
  return (
    <ul className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
      {stations.map((station) => (
        <li key={station.id}>
          <Link
            to={`/stations/${encodeURIComponent(station.id)}`}
            className="flex items-center justify-between rounded-lg border bg-card px-3 py-2.5 text-base no-underline! transition hover:bg-muted"
          >
            <span className="truncate">{station.name}</span>

            <ChevronRight
              size={16}
              className="shrink-0 text-muted-foreground"
            />
          </Link>
        </li>
      ))}
    </ul>
  );
}

// 路線を並べ、選んだ路線の駅だけを開いて見せる
function RailwayAccordion({ groups }: { groups: Group[] }) {
  // 開いた路線は URL に残す（駅の時刻表から戻ったとき、同じ路線を開いたままにする）
  const [params, setParams] = useSearchParams();
  const open = params.get("route");

  return (
    <Accordion
      value={open ? [open] : []}
      onValueChange={(value) => {
        const [route] = value as string[];
        setParams(route ? { route } : {}, { replace: true });
      }}
      className="gap-3"
    >
      {groups.map(({ railway, stations }) => (
        <AccordionItem
          key={railway.id}
          value={railway.id}
          className="rounded-xl border bg-card px-4"
        >
          <AccordionTrigger className="items-center py-3 text-base font-semibold hover:no-underline">
            <span className="flex items-center gap-3">
              <RailwayBadge railway={railway} className="size-8 text-sm" />
              {railway.name}
            </span>
          </AccordionTrigger>

          <AccordionContent className="pb-4">
            <StationGrid stations={stations} />
          </AccordionContent>
        </AccordionItem>
      ))}
    </Accordion>
  );
}

// 駅名で探したときの結果。路線をまたいで探せるよう、一致した駅を路線ごとに並べる
function SearchResult({
  groups,
  keyword,
}: {
  groups: Group[];
  keyword: string;
}) {
  if (groups.length === 0) {
    return (
      <p className="text-muted-foreground">
        「{keyword}」を含む駅は見つかりませんでした。
      </p>
    );
  }

  return groups.map(({ railway, stations }) => (
    <section key={railway.id} className="space-y-3">
      <h2 className="flex items-center gap-2 text-lg font-semibold">
        <RailwayBadge railway={railway} />
        {railway.name}
      </h2>

      <StationGrid stations={stations} />
    </section>
  ));
}

export default function Stations() {
  const [keyword, setKeyword] = useState("");

  const railways = useRailways();
  const stations = useQuery({
    queryKey: ["stations"],
    queryFn: api.getAllStations,
  });

  const word = keyword.trim();

  // 路線ごとに、駅名に検索語を含む駅だけを残す（検索語が空ならすべての駅）
  const groups = useMemo(
    () =>
      (railways.data ?? [])
        .map((railway) => ({
          railway,
          stations: (stations.data ?? []).filter(
            (s) => railwayIdOf(s.id) === railway.id && s.name.includes(word),
          ),
        }))
        .filter((g) => g.stations.length > 0),
    [railways.data, stations.data, word],
  );

  if (railways.isPending || stations.isPending) {
    return <Loading />;
  }

  if (railways.error || stations.error) {
    return <Error />;
  }

  return (
    <div className="space-y-8">
      <PageTitle title="駅・時刻表" />
      <div className="space-y-4">
        <h1 className="text-3xl font-bold">駅・時刻表</h1>

        <div className="relative max-w-md">
          <Search
            size={18}
            className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted-foreground"
          />

          <Input
            type="search"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="駅名で探す（例: 新宿）"
            aria-label="駅名"
            className="h-11 pl-10 text-base"
          />
        </div>
      </div>

      {word === "" ? (
        <RailwayAccordion groups={groups} />
      ) : (
        <SearchResult groups={groups} keyword={word} />
      )}
    </div>
  );
}
