import type { RailDirection } from "@/types";

import SegmentedControl from "@/components/SegmentedControl";

// 駅の一覧で、駅の時刻表を開く方向を選ぶ。方向が1つ以下の路線では出さない。
// 一覧の中身は変わらず、開く時刻表の方向だけが変わるので、タブではなくラジオボタンにする
export default function DirectionTabs({
  directions,
  value,
  onChange,
}: {
  directions: RailDirection[];
  value?: string;
  onChange(direction: string): void;
}) {
  if (directions.length <= 1) {
    return null;
  }

  return (
    <SegmentedControl
      options={directions.map((d) => ({ value: d.id, label: d.name }))}
      value={value}
      onChange={onChange}
      label="時刻表の方向"
      className="h-10 w-full sm:w-fit"
      optionClassName="px-4"
    />
  );
}

// 選んだ方向（URL の値）が路線の方向に無ければ、最初の方向にする
export function selectDirection(
  directions: RailDirection[],
  value: string | null,
) {
  return directions.some((d) => d.id === value)
    ? (value ?? undefined)
    : directions[0]?.id;
}
