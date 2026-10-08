import { Link, NavLink } from "react-router-dom";
import { TrainFront } from "lucide-react";

import { NAV_ITEMS } from "@/components/nav";

import { cn } from "@/lib/utils";

export default function Header() {
  return (
    <header className="sticky top-0 z-40 border-b bg-card/95 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between gap-4 px-4 sm:h-16 sm:px-6">
        <Link
          to="/"
          className="flex items-center gap-2.5 transition-opacity hover:opacity-80"
        >
          <span className="rounded-lg bg-primary p-1.5 text-primary-foreground">
            <TrainFront size={20} />
          </span>

          <span className="font-bold tracking-tight">都営 運行情報</span>
        </Link>

        {/* スマホでは画面下のタブバー（BottomNav）を使う */}
        <nav aria-label="メイン" className="hidden sm:block">
          <ul className="flex items-center gap-1">
            {NAV_ITEMS.map(({ to, label, icon: Icon }) => (
              <li key={to}>
                <NavLink
                  to={to}
                  end={to === "/"}
                  className={({ isActive }) =>
                    cn(
                      "flex items-center gap-1.5 rounded-md px-3 py-2 text-sm whitespace-nowrap transition",
                      isActive
                        ? "bg-muted font-medium text-foreground"
                        : "text-muted-foreground hover:bg-muted hover:text-foreground",
                    )
                  }
                >
                  <Icon size={16} />
                  {label}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>
      </div>
    </header>
  );
}
