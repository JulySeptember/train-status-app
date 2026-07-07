import { Outlet } from "react-router-dom";

import Header from "./Header";
import Footer from "./Footer";

export default function Layout() {
  return (
    <div className="min-h-screen bg-[#0d1117]">
      <Header />

      <main className="mx-auto w-full max-w-[1600px] px-6 py-6">
        <Outlet />
      </main>

      <Footer />
    </div>
  );
}
