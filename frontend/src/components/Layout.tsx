import { Outlet } from "react-router-dom";

import Header from "./Header";
import Footer from "./Footer";
import BottomNav from "./BottomNav";

export default function Layout() {
  return (
    <>
      <Header />

      {/* ヘッダー・フッターと同じ幅（max-w-6xl）に揃える */}
      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 sm:px-6 sm:py-8">
        <Outlet />
      </main>

      <Footer />

      {/* スマホでは、タブバーの高さだけ下に余白を空けてフッターが隠れないようにする */}
      <div
        aria-hidden
        className="h-[calc(3.5rem+env(safe-area-inset-bottom))] sm:hidden"
      />
      <BottomNav />
    </>
  );
}
