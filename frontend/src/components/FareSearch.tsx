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
  const [fromOpen, setFromOpen] = useState(false);
  const [toOpen, setToOpen] = useState(false);

  const [fromKeyword, setFromKeyword] = useState("");
  const [toKeyword, setToKeyword] = useState("");

  const fromStations = useMemo(
    () => stations.filter((s) => s.name.includes(fromKeyword)),
    [stations, fromKeyword],
  );

  const toStations = useMemo(
    () => stations.filter((s) => s.name.includes(toKeyword)),
    [stations, toKeyword],
  );

  const fromName =
    stations.find((s) => s.id === fromId)?.name ?? "出発駅を選択";

  const toName = stations.find((s) => s.id === toId)?.name ?? "到着駅を選択";

  return (
    <div className="grid gap-4 md:grid-cols-4">
      <Popover open={fromOpen} onOpenChange={setFromOpen}>
        <PopoverTrigger>
          <Button
            variant="outline"
            role="combobox"
            className="justify-between w-full"
          >
            {fromName}
            <ChevronsUpDown className="opacity-50" />
          </Button>
        </PopoverTrigger>

        <PopoverContent className="w-87.5 p-0">
          <Command>
            <CommandInput
              placeholder="出発駅を検索..."
              value={fromKeyword}
              onValueChange={setFromKeyword}
            />

            <CommandList>
              <CommandEmpty>駅が見つかりません</CommandEmpty>

              <CommandGroup>
                {fromStations.map((station) => (
                  <CommandItem
                    key={station.id}
                    value={station.name}
                    onSelect={() => {
                      onFromChange(station.id);
                      setFromOpen(false);
                      setFromKeyword("");
                    }}
                  >
                    <Check
                      className={cn(
                        "mr-2 h-4 w-4",
                        fromId === station.id ? "opacity-100" : "opacity-0",
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

      <Popover open={toOpen} onOpenChange={setToOpen}>
        <PopoverTrigger>
          <Button
            variant="outline"
            role="combobox"
            className="justify-between w-full"
          >
            {toName}
            <ChevronsUpDown className="opacity-50" />
          </Button>
        </PopoverTrigger>

        <PopoverContent className="w-87.5 p-0">
          <Command>
            <CommandInput
              placeholder="到着駅を検索..."
              value={toKeyword}
              onValueChange={setToKeyword}
            />

            <CommandList>
              <CommandEmpty>駅が見つかりません</CommandEmpty>

              <CommandGroup>
                {toStations.map((station) => (
                  <CommandItem
                    key={station.id}
                    value={station.name}
                    onSelect={() => {
                      onToChange(station.id);
                      setToOpen(false);
                      setToKeyword("");
                    }}
                  >
                    <Check
                      className={cn(
                        "mr-2 h-4 w-4",
                        toId === station.id ? "opacity-100" : "opacity-0",
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

      <Button disabled={!fromId || !toId} onClick={onSearch}>
        運賃検索
      </Button>

      <Button type="button" variant="outline" onClick={onReset}>
        リセット
      </Button>
    </div>
  );
}
