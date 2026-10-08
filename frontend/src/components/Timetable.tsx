import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { Link } from "react-router-dom";
import { Clock3, ArrowRight } from "lucide-react";

import {
  type DirectionTimetable,
  type Timetable as TimetableEntry,
} from "@/types";
import { nowServiceMinutes, timeToServiceMinutes } from "@/lib/time";
import { cn } from "@/lib/utils";

import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";

type Props = {
  weekday?: DirectionTimetable;
  saturday?: DirectionTimetable;
  holiday?: DirectionTimetable;
  saturdayHoliday?: DirectionTimetable;
  trainLocationAvailable: boolean;
};

type Tab = "weekday" | "saturday" | "holiday" | "saturdayHoliday";

// Tailwind の lg と同じ幅。PC 表示ではダイヤを並べ、スマホ表示ではタブで1つずつ出す
const desktopQuery = window.matchMedia("(min-width: 64rem)");

// 時刻表は1駅で数百行あるので、スマホ用と PC 用の両方を描いて片方を CSS で隠すと遅い。
// 画面の幅に合う方だけを描く
function useIsDesktop() {
  return useSyncExternalStore(
    (onChange) => {
      desktopQuery.addEventListener("change", onChange);
      return () => desktopQuery.removeEventListener("change", onChange);
    },
    () => desktopQuery.matches,
  );
}

// 次に発車する列車の位置。最終列車が出たあとは -1。
// 0時台・1時台の列車は運行日の最後に並ぶので、文字列ではなく運行日の経過分で比べる
function nextTrainIndex(trains: TimetableEntry[], now: number): number {
  let index = -1;
  let best = Infinity;

  trains.forEach((train, i) => {
    const minutes = timeToServiceMinutes(train.time);
    if (minutes !== null && minutes >= now && minutes < best) {
      best = minutes;
      index = i;
    }
  });

  return index;
}

// 本日のダイヤのとき、現在時刻（運行日の経過分）を30秒ごとに更新して返す
function useServiceNow(enabled: boolean): number | null {
  const [now, setNow] = useState(() => nowServiceMinutes());

  useEffect(() => {
    if (!enabled) {
      return;
    }

    const id = setInterval(() => setNow(nowServiceMinutes()), 30 * 1000);
    return () => clearInterval(id);
  }, [enabled]);

  return enabled ? now : null;
}

function TimetableCard({
  title,
  timetable,
  trainLocationAvailable,
}: {
  title: string;
  timetable?: DirectionTimetable;
  trainLocationAvailable: boolean;
}) {
  // 列車番号は平日・土休日ダイヤで使い回されるため、列車位置へのリンクは本日のダイヤに限る
  const linkable = trainLocationAvailable && !!timetable?.isToday;

  const now = useServiceNow(!!timetable?.isToday);
  const nextIndex =
    now !== null && timetable?.timetables
      ? nextTrainIndex(timetable.timetables, now)
      : -1;

  const nextRowRef = useRef<HTMLElement | null>(null);

  // 時刻表を開いたときと方面を切り替えたときに、次に発車する列車までスクロールする。
  // データの再取得や時刻の更新ではスクロールしない
  const scrollKey = timetable
    ? `${timetable.calendar}|${timetable.railDirection}`
    : "";

  useEffect(() => {
    nextRowRef.current?.scrollIntoView({ block: "center" });
  }, [scrollKey]);

  return (
    <div className="overflow-hidden rounded-xl border border-border bg-background">
      <div className="flex items-center gap-2 border-b border-border px-5 py-4">
        <Clock3 size={18} className="text-brand" />

        <h3 className="font-semibold text-foreground">{title}</h3>

        {timetable?.isToday && (
          <span className="rounded-full bg-primary/20 px-2 py-0.5 text-xs text-brand">
            本日
          </span>
        )}
      </div>

      <div className="divide-y divide-border">
        {timetable?.timetables?.length ? (
          timetable.timetables.map((train, i) => {
            // 各停（普通）以外の種別は色を変えて目立たせる
            const isLocal = train.trainTypeId.endsWith(".Local");

            const isNext = i === nextIndex;
            const rowRef = isNext
              ? (el: HTMLElement | null) => {
                  nextRowRef.current = el;
                }
              : undefined;
            // scroll-mt-20: 固定ヘッダー（h-16）に行が隠れないようにする
            const rowClass = `scroll-mt-20 px-5 py-4 ${
              isNext ? "border-l-4 border-l-brand bg-brand/10" : ""
            }`;

            const content = (
              <div>
                <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                  <p className="text-2xl font-bold text-foreground">
                    {train.time}
                  </p>

                  {isNext && (
                    <span className="rounded-full bg-brand/20 px-2 py-0.5 text-xs font-semibold text-brand">
                      次発
                    </span>
                  )}

                  {train.trainType && (
                    <span
                      className={
                        isLocal
                          ? "rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground"
                          : "rounded border border-warning/60 bg-warning/15 px-1.5 py-0.5 text-xs font-semibold text-warning"
                      }
                    >
                      {train.trainType}
                    </span>
                  )}

                  {train.destination && (
                    <p className="font-medium text-foreground">
                      {train.destination}行
                    </p>
                  )}
                </div>

                <p className="mt-1 text-sm text-muted-foreground">
                  列車番号 {train.trainNumber}
                </p>
              </div>
            );

            if (!linkable) {
              return (
                <div
                  key={`${train.time}-${train.trainId}`}
                  ref={rowRef}
                  className={rowClass}
                >
                  {content}
                </div>
              );
            }

            return (
              <Link
                key={`${train.time}-${train.trainId}`}
                to={`/trains/${encodeURIComponent(train.trainId)}`}
                ref={rowRef}
                className={`flex items-center justify-between transition hover:bg-card ${rowClass}`}
              >
                {content}

                <ArrowRight size={18} className="text-muted-foreground" />
              </Link>
            );
          })
        ) : (
          <div className="py-10 text-center text-muted-foreground">
            データがありません
          </div>
        )}
      </div>
    </div>
  );
}

export default function Timetable({
  weekday,
  saturday,
  holiday,
  saturdayHoliday,
  trainLocationAvailable,
}: Props) {
  // 「土休日」にまとめた路線と、「土曜・休日」が別ダイヤの路線がある
  const cards: { tab: Tab; title: string; timetable?: DirectionTimetable }[] =
    saturdayHoliday
      ? [
          { tab: "weekday", title: "平日", timetable: weekday },
          {
            tab: "saturdayHoliday",
            title: "土休日",
            timetable: saturdayHoliday,
          },
        ]
      : [
          { tab: "weekday", title: "平日", timetable: weekday },
          { tab: "saturday", title: "土曜", timetable: saturday },
          { tab: "holiday", title: "休日", timetable: holiday },
        ];

  // スマホ表示では、本日のダイヤのタブを最初に開く
  const [tab, setTab] = useState<Tab>(
    () => cards.find((c) => c.timetable?.isToday)?.tab ?? "weekday",
  );

  const isDesktop = useIsDesktop();

  if (isDesktop) {
    return (
      <div
        className={cn(
          "grid gap-6",
          cards.length === 2 ? "grid-cols-2" : "grid-cols-3",
        )}
      >
        {cards.map((c) => (
          <TimetableCard
            key={c.tab}
            title={c.title}
            timetable={c.timetable}
            trainLocationAvailable={trainLocationAvailable}
          />
        ))}
      </div>
    );
  }

  return (
    <div>
      <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)}>
        <TabsList className="mb-3 h-10! w-full">
          {cards.map((c) => (
            <TabsTrigger key={c.tab} value={c.tab}>
              {c.title}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      {cards
        .filter((c) => c.tab === tab)
        .map((c) => (
          <TimetableCard
            key={c.tab}
            title={c.title}
            timetable={c.timetable}
            trainLocationAvailable={trainLocationAvailable}
          />
        ))}
    </div>
  );
}
