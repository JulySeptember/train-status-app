import type React from "react";
import { createBrowserRouter } from "react-router-dom";

import Layout from "@/components/Layout";
import Loading from "@/components/Loading";
import { loadLazyPage } from "@/lib/lazyPage";

import Home from "@/pages/Home";
import Route from "@/pages/Route";
import Station from "@/pages/Station";
import Stations from "@/pages/Stations";
import Train from "@/pages/Train";
import Journey from "@/pages/Journey";
import NotFound from "@/pages/NotFound";

// 開く人の少ない画面は、開いたときに読み込む（最初に読み込む JS を小さくする）。
// 読み込むまでは前の画面を出したままにする（React Router の lazy）。
// 読み込めなかったとき（デプロイで前の版の JS が消えたときなど）の扱いは loadLazyPage
const lazyPage = (load: () => Promise<{ default: React.ComponentType }>) => ({
  lazy: async () => ({ Component: await loadLazyPage(load) }),
});

export const router = createBrowserRouter([
  {
    element: <Layout />,
    // 遅延読み込みの画面を直接開いたときは、読み込むまでこれを出す
    hydrateFallbackElement: <Loading />,
    children: [
      {
        path: "/",
        element: <Home />,
      },
      {
        path: "/routes/:routeId",
        element: <Route />,
      },
      {
        path: "/stations",
        element: <Stations />,
      },
      {
        path: "/stations/:stationId",
        element: <Station />,
      },
      {
        path: "/trains/:trainId",
        element: <Train />,
      },
      {
        path: "/journeys",
        element: <Journey />,
      },
      {
        path: "/chat",
        ...lazyPage(() => import("@/pages/Chat")),
      },
      {
        path: "/license",
        ...lazyPage(() => import("@/pages/License")),
      },
      {
        path: "/account",
        ...lazyPage(() => import("@/pages/Account")),
      },
      {
        path: "/privacy",
        ...lazyPage(() => import("@/pages/Privacy")),
      },
    ],
  },
  {
    path: "*",
    element: <NotFound />,
  },
]);
