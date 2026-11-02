import { useId } from "react";

import { cn } from "@/lib/utils";

type Option<T extends string> = {
  value: T;
  label: string;
};

type Props<T extends string> = {
  options: Option<T>[];
  value?: T;
  // T は options と value から決める（setState をそのまま渡したときに string に広がらないように）
  onChange(value: NoInfer<T>): void;

  // グループの名前（読み上げに使う）。例: "時刻の指定"
  label: string;

  className?: string;
  optionClassName?: string;
};

// 見た目はタブと同じで、中身は radiogroup の選択肢。切り替える中身（tabpanel）の無い選択に使う。
// ネイティブのラジオボタンなので、矢印キーで選べる
export default function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
  label,
  className,
  optionClassName,
}: Props<T>) {
  const name = useId();

  return (
    <div
      role="radiogroup"
      aria-label={label}
      className={cn(
        "inline-flex h-8 w-fit items-center justify-center rounded-lg bg-muted p-[3px] text-muted-foreground",
        className,
      )}
    >
      {options.map((option) => (
        <label
          key={option.value}
          className={cn(
            "relative inline-flex h-full flex-1 cursor-pointer items-center justify-center rounded-md border border-transparent px-1.5 py-0.5 text-sm font-medium whitespace-nowrap text-muted-foreground transition-all hover:text-foreground",
            "has-checked:border-input has-checked:bg-input/30 has-checked:text-foreground",
            "has-focus-visible:border-ring has-focus-visible:ring-[3px] has-focus-visible:ring-ring/50",
            optionClassName,
          )}
        >
          <input
            type="radio"
            name={name}
            value={option.value}
            checked={option.value === value}
            onChange={() => onChange(option.value)}
            // 透明にしてラベル全体に重ね、押した位置がラジオボタンそのものになるようにする
            className="absolute inset-0 cursor-pointer appearance-none rounded-md opacity-0"
          />
          {option.label}
        </label>
      ))}
    </div>
  );
}
