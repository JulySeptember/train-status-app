import { MapPin, Train, Timer } from "lucide-react";

import { type TrainLocation as TrainLocationType } from "@/types";

import RailwayBadge from "@/components/RailwayBadge";

import { railwayIdOf, useRailway } from "@/lib/railways";

type Props = {
  train: TrainLocationType;
};

export default function TrainLocation({ train }: Props) {
  const railway = useRailway(railwayIdOf(train.trainId));

  return (
    <section className="overflow-hidden rounded-xl border border-border bg-card">
      <div className="border-b border-border px-6 py-5">
        <div className="flex items-center gap-3">
          <Train className="text-brand" size={20} />

          <div>
            <h2 className="text-lg font-semibold text-foreground">
              列車現在位置
            </h2>

            <p className="text-sm text-muted-foreground">
              列車番号 {train.trainNumber}
            </p>
          </div>
        </div>
      </div>

      <div className="space-y-6 p-6">
        <div className="rounded-xl border border-border bg-background p-5">
          <div className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">
            路線
          </div>

          <p className="flex items-center gap-2 text-xl font-semibold text-foreground">
            <RailwayBadge railway={railway} />
            {train.railway}
          </p>
        </div>

        <div className="rounded-xl border border-border bg-background p-6">
          {train.stopped ? (
            <div className="flex flex-col items-center gap-3">
              <div className="h-5 w-5 rounded-full border-4 border-card bg-brand" />

              <div className="flex items-center gap-2 text-brand">
                <MapPin size={18} />

                <span className="font-medium">
                  {train.fromStation} に停車中
                </span>
              </div>
            </div>
          ) : (
            <>
              <div className="mb-6 flex items-center justify-between text-sm text-muted-foreground">
                <span>{train.fromStation}</span>

                <span>{train.toStation}</span>
              </div>

              <div className="relative">
                <div className="h-1 rounded-full bg-border" />

                <div className="absolute left-1/2 top-1/2 h-5 w-5 -translate-x-1/2 -translate-y-1/2 rounded-full border-4 border-card bg-brand" />
              </div>

              <div className="mt-5 flex items-center justify-center gap-2 text-brand">
                <MapPin size={18} />

                <span className="font-medium">
                  {train.fromStation} → {train.toStation} を走行中
                </span>
              </div>
            </>
          )}
        </div>

        <div className="rounded-xl border border-border bg-background p-5">
          <div className="flex items-center gap-2 text-muted-foreground">
            <Timer size={18} />

            <span>遅延時間</span>
          </div>

          <p className="mt-3 text-3xl font-bold text-foreground">
            {train.delay}
            <span className="ml-2 text-lg font-normal text-muted-foreground">
              秒
            </span>
          </p>
        </div>
      </div>
    </section>
  );
}
