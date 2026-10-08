import { type Railway } from "@/types";

import { FALLBACK_RAILWAY_COLOR, textColorOn } from "@/lib/railways";
import { cn } from "@/lib/utils";

type Props = {
  railway?: Railway;
  className?: string;
};

// 路線記号（A・I・S・E など）を路線の色で表示する。記号が無い路線は色の点だけにする
export default function RailwayBadge({ railway, className }: Props) {
  const color = railway?.color || FALLBACK_RAILWAY_COLOR;

  return (
    <span
      aria-hidden
      className={cn(
        "inline-flex size-7 shrink-0 items-center justify-center rounded-md text-xs leading-none font-bold",
        className,
      )}
      style={{ backgroundColor: color, color: textColorOn(color) }}
    >
      {railway?.lineCode}
    </span>
  );
}
