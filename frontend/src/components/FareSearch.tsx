import { type Station } from "@/types";

import { Button } from "@/components/ui/button";

import StationSelect from "@/components/StationSelect";

type Props = {
  stations: Station[];

  fromId: string;
  toId: string;

  onFromChange(id: string): void;
  onToChange(id: string): void;

  onSearch(): void;
  onReset(): void;
};

export default function FareSearch({
  stations,
  fromId,
  toId,
  onFromChange,
  onToChange,
  onSearch,
  onReset,
}: Props) {
  return (
    <div className="grid gap-4 md:grid-cols-4">
      <StationSelect
        stations={stations}
        value={fromId}
        onChange={onFromChange}
        label="出発駅"
      />

      <StationSelect
        stations={stations}
        value={toId}
        onChange={onToChange}
        label="到着駅"
      />

      <Button disabled={!fromId || !toId} onClick={onSearch}>
        運賃検索
      </Button>

      <Button type="button" variant="outline" onClick={onReset}>
        リセット
      </Button>
    </div>
  );
}
