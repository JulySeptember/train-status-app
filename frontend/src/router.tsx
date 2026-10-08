import { createBrowserRouter } from "react-router-dom";

import Layout from "@/components/Layout";

import Home from "@/pages/Home";
import Route from "@/pages/Route";
import Station from "@/pages/Station";
import Stations from "@/pages/Stations";
import Train from "@/pages/Train";
import Fare from "@/pages/Fare";
import Journey from "@/pages/Journey";
import Chat from "@/pages/Chat";
import NotFound from "@/pages/NotFound";
import License from "./pages/License";

export const router = createBrowserRouter([
  {
    element: <Layout />,
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
        element: <Chat />,
      },
      {
        path: "/fares",
        element: <Fare />,
      },
      {
        path: "/license",
        element: <License />,
      },
    ],
  },
  {
    path: "*",
    element: <NotFound />,
  },
]);
