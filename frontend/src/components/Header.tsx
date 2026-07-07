import { Link } from "react-router-dom";

import {
  NavigationMenu,
  NavigationMenuItem,
  NavigationMenuLink,
  NavigationMenuList,
} from "@/components/ui/navigation-menu";

export default function Header() {
  return (
    <header className="border-b">
      <div className="container mx-auto flex h-16 items-center justify-between">
        <Link to="/" className="text-xl font-bold">
          Train Status App
        </Link>

        <NavigationMenu>
          <NavigationMenuList>
            <NavigationMenuItem>
              <NavigationMenuLink render={<Link to="/" />}>
                Home
              </NavigationMenuLink>
            </NavigationMenuItem>
            <NavigationMenuItem>
              <NavigationMenuLink render={<Link to="/fares" />}>
                運賃検索
              </NavigationMenuLink>
            </NavigationMenuItem>
          </NavigationMenuList>
        </NavigationMenu>
      </div>
    </header>
  );
}
