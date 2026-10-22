import { useQuery } from "@tanstack/react-query";

import { api } from "@/api";
import { type Railway } from "@/types";

// ODPT が路線の色を配信していない路線に補う色（つくばエクスプレスの赤）
const RAILWAY_COLORS: Record<string, string> = {
  "odpt.Railway:MIR.TsukubaExpress": "#e60012",
};

// ODPT が路線記号を配信していない路線に補う記号。記号の決まっていない八高線・相鉄直通線は補わない
const LINE_CODES: Record<string, string> = {
  "odpt.Railway:JR-East.JobanRapid": "JJ",
  "odpt.Railway:JR-East.Keiyo": "JE",
  "odpt.Railway:JR-East.SaikyoKawagoe": "JA",
  "odpt.Railway:JR-East.Takasaki": "JU",
  "odpt.Railway:JR-East.Utsunomiya": "JU",
  "odpt.Railway:Keikyu.Main": "KK",
};

// 運行情報の一覧に出さない路線。都内の駅が1つだけで、都内に絞った時刻表に列車が無い
export const HIDDEN_RAILWAYS = new Set([
  "odpt.Railway:JR-East.Chuo", // 中央本線（高尾〜塩尻。都内は高尾だけ）
  "odpt.Railway:Seibu.Yamaguchi", // 山口線（多摩湖〜西武球場前。都内は多摩湖だけ）
]);

async function getRailways(): Promise<Railway[]> {
  const railways = await api.getRoutes();
  return railways.map((r) => ({
    ...r,
    color: r.color || RAILWAY_COLORS[r.id],
    lineCode: r.lineCode || LINE_CODES[r.id],
  }));
}

// 路線一覧（GET /api/routes）。路線の色・記号を引くのに、どの画面からも同じキャッシュを使う
export function useRailways() {
  return useQuery({
    queryKey: ["routes"],
    queryFn: getRailways,
    staleTime: Infinity,
  });
}

export type OperatorGroup<T> = {
  // 事業者の ID（例: odpt.Operator:Toei）と表示名
  operator: string;
  name: string;
  items: T[];
};

// 路線（や路線ごとの項目）を事業者ごとにまとめる。事業者・項目は最初に現れた順に並べる
export function groupByOperator<T>(
  items: T[],
  railwayOf: (item: T) => Railway,
): OperatorGroup<T>[] {
  const groups = new Map<string, OperatorGroup<T>>();

  for (const item of items) {
    const railway = railwayOf(item);
    const operator = railway.operator ?? "";

    let group = groups.get(operator);
    if (!group) {
      group = { operator, name: railway.operatorName ?? "", items: [] };
      groups.set(operator, group);
    }
    group.items.push(item);
  }

  return [...groups.values()];
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

// 同じ記号・色の路線（京王線の支線など）は記号を1つだけ出す
export function uniqueBadges(railways: Railway[]) {
  const seen = new Set<string>();
  return railways.filter((r) => {
    const key = `${r.lineCode ?? ""}-${r.color ?? r.id}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}
