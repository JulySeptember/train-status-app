import { useQuery } from "@tanstack/react-query";

import { api } from "@/api";
import { type Railway } from "@/types";

// 路線一覧（GET /api/routes）。路線の色・記号を引くのに、どの画面からも同じキャッシュを使う
export function useRailways() {
  return useQuery({
    queryKey: ["routes"],
    queryFn: api.getRoutes,
    staleTime: Infinity,
  });
}

// 路線ID から路線を引く。一覧の取得前や、一覧に無い路線では undefined
export function useRailway(id: string | undefined): Railway | undefined {
  const { data } = useRailways();
  return id ? data?.find((r) => r.id === id) : undefined;
}

// 駅ID（odpt.Station:<事業者>.<路線>.<駅>）・列車ID（odpt.Train:<事業者>.<路線>.<番号>）から路線ID を作る
export function railwayIdOf(id: string): string | undefined {
  const [, rest] = id.split(":");
  const parts = rest?.split(".");

  if (!parts || parts.length < 3) {
    return undefined;
  }

  return `odpt.Railway:${parts[0]}.${parts[1]}`;
}

// 路線の色が配信されていない路線（都電荒川線など）に使う色
export const FALLBACK_RAILWAY_COLOR = "#8b949e";

// 路線の色の上に載せる文字の色。明るい色（新宿線の黄緑など）には黒、暗い色には白を使う
export function textColorOn(hex: string): string {
  const n = parseInt(hex.slice(1), 16);
  const [r, g, b] = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map((v) => {
    const c = v / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  const luminance = 0.2126 * r + 0.7152 * g + 0.0722 * b;

  // 白と黒のうち、コントラスト比が大きいほう
  return 1.05 / (luminance + 0.05) > (luminance + 0.05) / 0.05
    ? "#ffffff"
    : "#0d1117";
}
