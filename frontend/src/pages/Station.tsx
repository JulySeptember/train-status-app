import { useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";

import { api } from "@/api";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import Timetable from "@/components/Timetable";
import PassengerTable from "@/components/PassengerTable";

import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { directionLabel } from "@/lib/odpt";

export default function Station() {
  const { stationId = "" } = useParams();

  const { data, isPending, error } = useQuery({
    queryKey: ["station", stationId],
    queryFn: () => api.getStation(stationId),
    staleTime: 5 * 60 * 1000,
  });

  const [direction, setDirection] = useState("");

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
      <div>
        <h1 className="text-3xl font-bold">{data.name}</h1>
      </div>

      <section className="space-y-4">
        <h2 className="text-xl font-semibold">時刻表</h2>

        <Tabs value={selectedDirection} onValueChange={setDirection}>
          <TabsList>
            {directions.map((d) => (
              <TabsTrigger key={d} value={d}>
                {directionLabel(d)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <Timetable
          weekday={weekday}
          saturday={saturday}
          holiday={holiday}
          saturdayHoliday={saturdayHoliday}
        />
      </section>

      <section className="space-y-4">
        <h2 className="text-xl font-semibold">乗降者数</h2>

        <PassengerTable passengers={data.passengers ?? []} />
      </section>
    </div>
  );
}
