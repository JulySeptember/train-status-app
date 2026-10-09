import type { RailDirection } from "@/types";

import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";

// 駅の一覧で、駅の時刻表を開く方向を選ぶタブ。方向が1つ以下の路線では出さない
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
    <Tabs value={value} onValueChange={onChange}>
      <TabsList className="h-10! w-full sm:w-fit">
        {directions.map((d) => (
          <TabsTrigger key={d.id} value={d.id} className="px-4">
            {d.name}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
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
