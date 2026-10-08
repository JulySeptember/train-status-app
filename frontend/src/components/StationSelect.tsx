import { useMemo, useState } from "react";
import { Check, ChevronsUpDown } from "lucide-react";

import { type Station } from "@/types";

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

type Props = {
  stations: Station[];
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

  const filtered = useMemo(
    () => stations.filter((s) => s.name.includes(keyword)),
    [stations, keyword],
  );

  const name = stations.find((s) => s.id === value)?.name ?? `${label}を選択`;

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
        {name}
        <ChevronsUpDown className="opacity-50" />
      </PopoverTrigger>

      <PopoverContent className="w-87.5 p-0">
        <Command>
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
                  value={station.name}
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

                  {station.name}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
