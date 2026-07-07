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
    <header className="sticky top-0 z-40 border-b border-[#30363d] bg-[#161b22]/95 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-[1600px] items-center justify-between px-8">
        <Link
          to="/"
          className="flex items-center gap-3 transition-opacity hover:opacity-80"
        >
          <div className="rounded-lg bg-[#58a6ff] p-2">
            <Train size={20} className="text-white" />
          </div>
        </Link>

        <NavigationMenu>
          <NavigationMenuList className="gap-2">
            <NavigationMenuItem>
              <NavigationMenuLink
                render={<Link to="/" />}
                className="rounded-md px-4 py-2 text-gray-300 transition hover:bg-[#21262d] hover:text-white"
              >
                Home
              </NavigationMenuLink>
            </NavigationMenuItem>

            <NavigationMenuItem>
              <NavigationMenuLink
                render={<Link to="/fares" />}
                className="rounded-md px-4 py-2 text-gray-300 transition hover:bg-[#21262d] hover:text-white"
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
