import { Outlet } from "react-router-dom";

import Header from "./Header";
import Footer from "./Footer";

export default function Layout() {
  return (
    <>
      <Header />

      <main className="container mx-auto px-6 py-8">
        <Outlet />
      </main>

      <Footer />
    </>
  );
}
