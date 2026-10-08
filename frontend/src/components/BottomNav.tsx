import { NavLink } from "react-router-dom";

import { NAV_ITEMS } from "@/components/nav";

import { cn } from "@/lib/utils";

// スマホ用の、画面下に固定するタブバー。PC ではヘッダーのナビゲーションを使う
export default function BottomNav() {
  return (
    <nav
      aria-label="メイン"
      className="fixed inset-x-0 bottom-0 z-40 border-t bg-card/95 pb-[env(safe-area-inset-bottom)] backdrop-blur sm:hidden"
    >
      <ul className="grid grid-cols-4">
        {NAV_ITEMS.map(({ to, label, icon: Icon }) => (
          <li key={to}>
            <NavLink
              to={to}
              end={to === "/"}
              className={({ isActive }) =>
                cn(
                  "flex h-14 flex-col items-center justify-center gap-0.5 text-[11px] transition",
                  isActive
                    ? "font-medium text-brand"
                    : "text-muted-foreground hover:text-foreground",
                )
              }
            >
              <Icon size={20} />
              {label}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  );
}
