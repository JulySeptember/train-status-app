import { Link } from "react-router-dom";
import { Train } from "lucide-react";

import {
  NavigationMenu,
  NavigationMenuItem,
  NavigationMenuLink,
  NavigationMenuList,
} from "@/components/ui/navigation-menu";

export default function Header() {
  return (
    <header className="sticky top-0 z-40 border-b border-border bg-card/95 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-4 sm:px-6">
        <Link
          to="/"
          className="flex items-center gap-3 transition-opacity hover:opacity-80"
        >
          <div className="rounded-lg bg-brand p-2">
            <Train size={20} className="text-foreground" />
          </div>
        </Link>

        <NavigationMenu>
          <NavigationMenuList className="gap-0 sm:gap-2">
            <NavigationMenuItem>
              <NavigationMenuLink
                render={<Link to="/" />}
                className="rounded-md px-2 py-2 text-sm whitespace-nowrap text-foreground/80 transition hover:bg-muted hover:text-foreground sm:px-4 sm:text-base"
              >
                Home
              </NavigationMenuLink>
            </NavigationMenuItem>

            <NavigationMenuItem>
              <NavigationMenuLink
                render={<Link to="/journeys" />}
                className="rounded-md px-2 py-2 text-sm whitespace-nowrap text-foreground/80 transition hover:bg-muted hover:text-foreground sm:px-4 sm:text-base"
              >
                経路検索
              </NavigationMenuLink>
            </NavigationMenuItem>

            <NavigationMenuItem>
              <NavigationMenuLink
                render={<Link to="/chat" />}
                className="rounded-md px-2 py-2 text-sm whitespace-nowrap text-foreground/80 transition hover:bg-muted hover:text-foreground sm:px-4 sm:text-base"
              >
                AI に聞く
              </NavigationMenuLink>
            </NavigationMenuItem>

            <NavigationMenuItem>
              <NavigationMenuLink
                render={<Link to="/fares" />}
                className="rounded-md px-2 py-2 text-sm whitespace-nowrap text-foreground/80 transition hover:bg-muted hover:text-foreground sm:px-4 sm:text-base"
              >
                運賃検索
              </NavigationMenuLink>
            </NavigationMenuItem>
          </NavigationMenuList>
        </NavigationMenu>
      </div>
    </header>
  );
}
