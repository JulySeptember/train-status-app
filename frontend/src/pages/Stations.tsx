import { useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ChevronRight, Search } from "lucide-react";

import { api } from "@/api";
import { type Railway, type StationSummary } from "@/types";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import RailwayBadge from "@/components/RailwayBadge";
import PageTitle from "@/components/PageTitle";
import DirectionTabs, { selectDirection } from "@/components/DirectionTabs";

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Input } from "@/components/ui/input";
import { groupByOperator, uniqueBadges, useRailways } from "@/lib/railways";
import { stationPath, usePrefetchStation } from "@/lib/station";

type Group = {
  railway: Railway;
  stations: StationSummary[];
};

function StationGrid({
  stations,
  direction,
}: {
  stations: StationSummary[];
  direction?: string;
}) {
  const prefetch = usePrefetchStation();

  return (
    <ul className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
      {stations.map((station) => (
        <li key={station.id}>
          <Link
            to={stationPath(station.id, direction)}
            {...prefetch(station.id)}
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

// 路線を並べ、選んだ路線の駅だけを開いて見せる。
// 駅の時刻表は方向ごとなので、駅を選ぶ前に方向も選べるようにする（選ばなければ最初の方向で開く）
function RailwayAccordion({
  groups,
  open,
  onOpen,
  direction,
  onDirection,
}: {
  groups: Group[];
  open: string | null;
  onOpen(route: string | null): void;
  direction: string | null;
  onDirection(direction: string): void;
}) {
  return (
    <Accordion
      value={open ? [open] : []}
      onValueChange={(value) => onOpen((value as string[])[0] ?? null)}
      className="gap-3"
    >
      {groups.map(({ railway, stations }) => {
        const directions = railway.directions ?? [];
        const selected = selectDirection(directions, direction);

        return (
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

            <AccordionContent className="space-y-3 pb-4">
              <DirectionTabs
                directions={directions}
                value={selected}
                onChange={onDirection}
              />

              <StationGrid stations={stations} direction={selected} />
            </AccordionContent>
          </AccordionItem>
        );
      })}
    </Accordion>
  );
}

// 事業者 → 路線の2段で並べる。事業者が1つ（都営だけ）なら路線だけを並べる
function OperatorAccordion({ groups }: { groups: Group[] }) {
  // 開いた事業者・路線は URL に残す（駅の時刻表から戻ったとき、同じ路線を開いたままにする）
  const [params, setParams] = useSearchParams();
  const route = params.get("route");
  const direction = params.get("direction");

  const operators = groupByOperator(groups, (g) => g.railway);

  // 路線を開いていれば、その路線の事業者も開く
  const operator =
    params.get("operator") ??
    groups.find((g) => g.railway.id === route)?.railway.operator ??
    null;

  // 方向は開いた路線のものなので、路線を変えたら選び直す
  const set = (next: {
    operator?: string | null;
    route?: string | null;
    direction?: string | null;
  }) => {
    const value: Record<string, string> = {};
    if (next.operator) value.operator = next.operator;
    if (next.route) value.route = next.route;
    if (next.direction) value.direction = next.direction;
    setParams(value, { replace: true });
  };

  if (operators.length <= 1) {
    return (
      <RailwayAccordion
        groups={groups}
        open={route}
        onOpen={(route) => set({ route })}
        direction={direction}
        onDirection={(direction) => set({ route, direction })}
      />
    );
  }

  return (
    <Accordion
      value={operator ? [operator] : []}
      onValueChange={(value) => set({ operator: (value as string[])[0] })}
      className="gap-3"
    >
      {operators.map((op) => (
        <AccordionItem
          key={op.operator}
          value={op.operator}
          className="rounded-xl border bg-card px-4"
        >
          <AccordionTrigger className="items-center py-3 hover:no-underline">
            <span className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2">
              <span className="text-lg font-semibold">{op.name}</span>

              {/* 閉じていても、どの路線があるか分かるように記号を並べる */}
              <span className="flex flex-wrap gap-1">
                {uniqueBadges(op.items.map((g) => g.railway)).map((railway) => (
                  <RailwayBadge
                    key={railway.id}
                    railway={railway}
                    className="size-5 text-[10px]"
                  />
                ))}
              </span>
            </span>
          </AccordionTrigger>

          <AccordionContent className="pb-4">
            <RailwayAccordion
              groups={op.items}
              open={route}
              onOpen={(route) => set({ operator: op.operator, route })}
              direction={direction}
              onDirection={(direction) =>
                set({ operator: op.operator, route, direction })
              }
            />
          </AccordionContent>
        </AccordionItem>
      ))}
    </Accordion>
  );
}

// 駅名で探したときの結果。同じ名前の駅を1行にまとめ、路線を選んで時刻表を開く
function SearchResult({
  stations,
  railways,
  keyword,
}: {
  stations: StationSummary[];
  railways: Map<string, Railway>;
  keyword: string;
}) {
  const prefetch = usePrefetchStation();

  const byName = useMemo(() => {
    const result = new Map<string, StationSummary[]>();
    for (const s of stations) {
      result.set(s.name, [...(result.get(s.name) ?? []), s]);
    }
    // 検索語と同じ名前の駅、検索語で始まる駅、それ以外の順に並べる
    const rank = (name: string) =>
      name === keyword ? 0 : name.startsWith(keyword) ? 1 : 2;
    return [...result].sort(([a], [b]) => rank(a) - rank(b));
  }, [stations, keyword]);

  if (byName.length === 0) {
    return (
      <p className="text-muted-foreground">
        「{keyword}」を含む駅は見つかりませんでした。
      </p>
    );
  }

  return (
    <ul className="space-y-3">
      {byName.map(([name, items]) => (
        <li key={name} className="space-y-2 rounded-xl border bg-card p-4">
          <h2 className="text-lg font-semibold">{name}</h2>

          <div className="flex flex-wrap gap-2">
            {items.map((station) => {
              const railway = railways.get(station.railwayId);

              return (
                <Link
                  key={station.id}
                  to={`/stations/${encodeURIComponent(station.id)}`}
                  {...prefetch(station.id)}
                  className="flex items-center gap-2 rounded-lg border bg-background px-3 py-2 text-sm no-underline! transition hover:bg-muted"
                >
                  <RailwayBadge railway={railway} className="size-6" />
                  {railway?.name ?? station.railwayId}
                </Link>
              );
            })}
          </div>
        </li>
      ))}
    </ul>
  );
}

export default function Stations() {
  const [keyword, setKeyword] = useState("");

  const railways = useRailways();
  const stations = useQuery({
    queryKey: ["stations"],
    queryFn: api.getAllStations,
  });

  const word = keyword.trim();

  const groups = useMemo(
    () =>
      (railways.data ?? [])
        .map((railway) => ({
          railway,
          stations: (stations.data ?? []).filter(
            (s) => s.railwayId === railway.id,
          ),
        }))
        .filter((g) => g.stations.length > 0),
    [railways.data, stations.data],
  );

  const railwayById = useMemo(
    () => new Map((railways.data ?? []).map((r) => [r.id, r])),
    [railways.data],
  );

  const matched = useMemo(
    () => (stations.data ?? []).filter((s) => s.name.includes(word)),
    [stations.data, word],
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
        <OperatorAccordion groups={groups} />
      ) : (
        <SearchResult
          stations={matched}
          railways={railwayById}
          keyword={word}
        />
      )}
    </div>
  );
}
