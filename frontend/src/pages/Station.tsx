import { useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, ChevronUp } from "lucide-react";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import Timetable from "@/components/Timetable";
import PassengerTable from "@/components/PassengerTable";
import RailwayBadge from "@/components/RailwayBadge";
import PageTitle from "@/components/PageTitle";

import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { directionLabel } from "@/lib/odpt";
import { railwayIdOf, useRailway } from "@/lib/railways";
import { stationQuery } from "@/lib/station";
import type { DirectionTimetable } from "@/types";

// 方向の名前（北行・上りなど）だけではどこへ行くのかわからないので、主な行先を添える。
// 本数の多い行先を2つまで出し、少ない行先（車庫行きなど）は出さない。
// 大江戸線の環状部のように行先の無い列車がほとんどの方向は、一部の列車の行先を出すと誤解されるので出さない
function mainDestinations(timetables: DirectionTimetable[]) {
  const counts = new Map<string, number>();
  let total = 0;
  let unknown = 0;

  for (const t of timetables) {
    for (const train of t.timetables) {
      total++;
      if (train.destination) {
        counts.set(train.destination, (counts.get(train.destination) ?? 0) + 1);
      } else {
        unknown++;
      }
    }
  }

  if (unknown > total / 2) {
    return [];
  }

  return [...counts]
    .filter(([, n]) => n >= total * 0.1)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 2)
    .map(([name]) => name);
}

// 「光が丘方面」のように方向の名前に行先が入っているときは、行先を添えない
function directionHint(direction: string, timetables: DirectionTimetable[]) {
  if (directionLabel(direction).endsWith("方面")) {
    return [];
  }

  return mainDestinations(
    timetables.filter((t) => t.railDirection === direction),
  );
}

// 狭い画面では、行先の名前の途中ではなく行先の区切りで折り返す
// （「羽田空港第１・第２ターミナル」のように名前に「・」を含む駅がある）
function Hint({ names }: { names: string[] }) {
  return names.map((name, i) => (
    <span key={name} className="inline-block">
      {name}
      {i < names.length - 1 ? "・" : "方面"}
    </span>
  ));
}

export default function Station() {
  const { stationId = "" } = useParams();

  const { data, isPending, error } = useQuery(stationQuery(stationId));

  const railway = useRailway(railwayIdOf(stationId));

  const [direction, setDirection] = useState("");
  const [showPassengers, setShowPassengers] = useState(false);

  if (isPending) {
    return <Loading />;
  }

  if (error || !data) {
    return <Error />;
  }

  const directions = [...new Set(data.timetables.map((t) => t.railDirection))];

  const selectedDirection = directions.includes(direction)
    ? direction
    : (directions[0] ?? "");

  const selectedHint = directionHint(selectedDirection, data.timetables);

  const weekday = data.timetables.find(
    (t) =>
      t.calendar === "odpt.Calendar:Weekday" &&
      t.railDirection === selectedDirection,
  );

  const saturday = data.timetables.find(
    (t) =>
      t.calendar === "odpt.Calendar:Saturday" &&
      t.railDirection === selectedDirection,
  );

  const holiday = data.timetables.find(
    (t) =>
      t.calendar === "odpt.Calendar:Holiday" &&
      t.railDirection === selectedDirection,
  );

  const saturdayHoliday = data.timetables.find(
    (t) =>
      t.calendar === "odpt.Calendar:SaturdayHoliday" &&
      t.railDirection === selectedDirection,
  );

  return (
    <div className="space-y-8">
      <PageTitle title={`${data.name}駅の時刻表`} />
      <div className="space-y-2">
        <h1 className="text-3xl font-bold text-foreground">{data.name}</h1>

        {railway && (
          <p className="flex items-center gap-2 text-foreground/80">
            <RailwayBadge railway={railway} className="size-6" />
            {railway.name}
          </p>
        )}
      </div>

      <section className="space-y-4">
        <h2 className="text-xl font-semibold text-foreground">時刻表</h2>

        <p className="text-sm text-muted-foreground">
          {data.trainLocationAvailable
            ? "本日のダイヤの列車を選択すると、現在位置を確認できます。"
            : "この路線は列車位置情報が提供されていません。"}
        </p>

        {/* 開いたときは最初の方向を出すので、ほかの方向を選べることがわかるよう、行先を添えて大きく並べる */}
        {directions.length > 1 && (
          <Tabs value={selectedDirection} onValueChange={setDirection}>
            <TabsList className="h-auto! w-full">
              {directions.map((d) => {
                const hint = directionHint(d, data.timetables);

                return (
                  <TabsTrigger
                    key={d}
                    value={d}
                    className="flex-col gap-0 py-1.5 whitespace-normal"
                  >
                    <span className="text-base">{directionLabel(d)}</span>
                    {hint.length > 0 && (
                      <span className="text-xs font-normal text-muted-foreground">
                        <Hint names={hint} />
                      </span>
                    )}
                  </TabsTrigger>
                );
              })}
            </TabsList>
          </Tabs>
        )}

        {directions.length === 1 && (
          <p className="font-medium text-foreground">
            {directionLabel(selectedDirection)}
            {selectedHint.length > 0 && `（${selectedHint.join("・")}方面）`}
          </p>
        )}

        <Timetable
          weekday={weekday}
          saturday={saturday}
          holiday={holiday}
          saturdayHoliday={saturdayHoliday}
          trainLocationAvailable={data.trainLocationAvailable}
        />
      </section>

      <section className="space-y-4">
        <button
          onClick={() => setShowPassengers((v) => !v)}
          className="flex w-full items-center justify-between rounded-xl border border-border bg-card px-5 py-4 transition hover:bg-muted"
        >
          <span className="text-xl font-semibold text-foreground">
            年間乗降人員
          </span>

          {showPassengers ? (
            <ChevronUp size={20} className="text-muted-foreground" />
          ) : (
            <ChevronDown size={20} className="text-muted-foreground" />
          )}
        </button>

        {showPassengers && (
          <div className="overflow-hidden rounded-xl border border-border bg-background p-5">
            <PassengerTable passengers={data.passengers ?? []} />
          </div>
        )}
      </section>
    </div>
  );
}
