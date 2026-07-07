import { createBrowserRouter } from "react-router-dom";

import Layout from "@/components/Layout";

import Home from "@/pages/Home";
import Route from "@/pages/Route";
import Station from "@/pages/Station";
import Train from "@/pages/Train";
import Fare from "@/pages/Fare";
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
        path: "/stations/:stationId",
        element: <Station />,
      },
      {
        path: "/trains/:trainNumber",
        element: <Train />,
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
