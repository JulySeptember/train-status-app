import { Outlet } from "react-router-dom";

import Header from "./Header";
import Footer from "./Footer";

export default function Layout() {
  return (
    <>
      <Header />

      {/* ヘッダー・フッターと同じ幅（max-w-6xl）に揃える */}
      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 sm:px-6 sm:py-8">
        <Outlet />
      </main>

      <Footer />
    </>
  );
}
