import { useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
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

export default function Station() {
  const { stationId = "" } = useParams();

  const { data, isPending, error } = useQuery(stationQuery(stationId));

  const railway = useRailway(railwayIdOf(stationId));

  // 方向は URL に残す。駅の一覧で方向を選んできたときは、その方向で開く
  const [params, setParams] = useSearchParams();
  const direction = params.get("direction") ?? "";
  const setDirection = (value: string) =>
    setParams({ direction: value }, { replace: true });
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

        <Tabs value={selectedDirection} onValueChange={setDirection}>
          <TabsList>
            {directions.map((d) => (
              <TabsTrigger key={d} value={d}>
                {railway?.directions?.find((r) => r.id === d)?.name ??
                  directionLabel(d)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

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
