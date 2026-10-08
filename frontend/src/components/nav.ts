import {
  Activity,
  MapPin,
  Route,
  Sparkles,
  type LucideIcon,
} from "lucide-react";

export type NavItem = {
  to: string;
  label: string;
  icon: LucideIcon;
};

// ヘッダー（PC）と画面下のタブバー（スマホ）で共通の、主な機能への入口。
// 運賃検索はデータが途中までしか無く、扱いを保留しているので載せない（/fares は残す）
export const NAV_ITEMS: NavItem[] = [
  { to: "/", label: "運行情報", icon: Activity },
  { to: "/journeys", label: "経路検索", icon: Route },
  { to: "/chat", label: "AI に聞く", icon: Sparkles },
  { to: "/stations", label: "駅・時刻表", icon: MapPin },
];
