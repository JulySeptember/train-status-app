import { useMemo, useState, type ReactNode } from "react";
import { Check, ChevronsUpDown } from "lucide-react";

import { Button } from "@/components/ui/button";

import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";

import { cn } from "@/lib/utils";

export type StationOption = {
  id: string;
  name: string;

  // 駅名の横に出す補足（路線の記号など）。同じ名前の選択肢を見分けるのに使う
  detail?: ReactNode;
  // 選べない選択肢（経路検索に使えない駅など）の理由
  disabledReason?: string;
};

type Props = {
  stations: StationOption[];
  value: string;
  onChange(id: string): void;

  // 例: "出発駅"。ボタンの初期表示と検索欄に使う
  label: string;

  disabled?: boolean;
};

export default function StationSelect({
  stations,
  value,
  onChange,
  label,
  disabled,
}: Props) {
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState("");

  const filtered = useMemo(() => {
    // 検索語と同じ名前の駅、検索語で始まる駅、それ以外の順に並べる
    const rank = (name: string) =>
      name === keyword ? 0 : name.startsWith(keyword) ? 1 : 2;
    return stations
      .filter((s) => s.name.includes(keyword))
      .sort((a, b) => rank(a.name) - rank(b.name));
  }, [stations, keyword]);

  const current = stations.find((s) => s.id === value);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      {/* Button をそのままトリガーにする（中に入れると button が入れ子になる） */}
      <PopoverTrigger
        render={
          <Button
            variant="outline"
            role="combobox"
            disabled={disabled}
            className="h-10 w-full justify-between text-base"
          />
        }
      >
        <span className="flex min-w-0 items-center gap-2 truncate">
          {current?.name ?? `${label}を選択`}
          {current?.detail}
        </span>
        <ChevronsUpDown className="opacity-50" />
      </PopoverTrigger>

      <PopoverContent className="w-87.5 p-0">
        {/* 絞り込みは駅名で行う（cmdk の絞り込みは value に入れた ID にも当たるので使わない） */}
        <Command shouldFilter={false}>
          <CommandInput
            placeholder={`${label}を検索...`}
            value={keyword}
            onValueChange={setKeyword}
          />

          <CommandList>
            <CommandEmpty>駅が見つかりません</CommandEmpty>

            <CommandGroup>
              {filtered.map((station) => (
                <CommandItem
                  key={station.id}
                  value={station.id}
                  disabled={!!station.disabledReason}
                  onSelect={() => {
                    onChange(station.id);
                    setOpen(false);
                    setKeyword("");
                  }}
                >
                  <Check
                    className={cn(
                      "mr-2 h-4 w-4",
                      value === station.id ? "opacity-100" : "opacity-0",
                    )}
                  />

                  <span className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1">
                    {station.name}
                    {station.detail}
                    {station.disabledReason && (
                      <span className="text-xs text-muted-foreground">
                        {station.disabledReason}
                      </span>
                    )}
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
