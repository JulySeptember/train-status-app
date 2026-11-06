import { type Journey } from "@/types";

import { timeToServiceMinutes } from "@/lib/time";

// 経路検索の結果の並び順。time は API の並び（早く着く順。到着時刻の指定では遅く出る順）のまま
export type JourneySort = "time" | "transfers" | "duration";

const SORTS: readonly JourneySort[] = ["time", "transfers", "duration"];

// 並び順は URL の sort に持たせる（再読み込み・共有しても同じ並びにする）。
// time は既定なので URL に書かない
export const SORT_PARAM = "sort";

export function readJourneySort(params: URLSearchParams): JourneySort {
  const sort = params.get(SORT_PARAM);
  return SORTS.find((s) => s === sort) ?? "time";
}

export function journeySortOptions(
  arriveBy: boolean,
): { value: JourneySort; label: string }[] {
  return [
    { value: "time", label: arriveBy ? "遅く出る順" : "早く着く順" },
    { value: "transfers", label: "乗り換えが少ない順" },
    { value: "duration", label: "所要時間が短い順" },
  ];
}

function durationMinutes(journey: Journey) {
  return (
    (timeToServiceMinutes(journey.arrivalTime) ?? 0) -
    (timeToServiceMinutes(journey.departureTime) ?? 0)
  );
}

// 並べ替えた新しい配列を返す。同じ値どうしは API の並び（time）を保つ（sort は安定）
export function sortJourneys(
  journeys: Journey[],
  sort: JourneySort,
): Journey[] {
  switch (sort) {
    case "time":
      return journeys;
    case "transfers":
      return [...journeys].sort((a, b) => a.transfers - b.transfers);
    case "duration":
      return [...journeys].sort(
        (a, b) =>
          durationMinutes(a) - durationMinutes(b) || a.transfers - b.transfers,
      );
  }
}
