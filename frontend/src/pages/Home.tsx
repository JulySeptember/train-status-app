import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";

import { api } from "@/api";

import Loading from "@/components/Loading";
import Error from "@/components/Error";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export default function Home() {
  const status = useQuery({
    queryKey: ["status"],
    queryFn: api.getStatus,
  });

  const routes = useQuery({
    queryKey: ["routes"],
    queryFn: api.getRoutes,
  });

  if (status.isPending || routes.isPending) {
    return <Loading />;
  }

  if (status.error || routes.error) {
    return <Error />;
  }

  return (
    <div className="space-y-8">
      <section>
        <h2 className="mb-4 text-2xl font-bold">運行情報</h2>

        <div className="space-y-3">
          {status.data?.map((item) => {
            const isNormal =
              item.status.includes("現在、15分以上の遅延はありません") ||
              item.status.includes("現在、１５分以上の遅延はありません");

            return (
              <Card key={item.railway}>
                <CardHeader>
                  <CardTitle>{item.railway}</CardTitle>
                </CardHeader>

                <CardContent>
                  <Badge
                    className={
                      isNormal
                        ? "border border-emerald-800 bg-emerald-900/50 text-white"
                        : "border border-red-800 bg-red-900/50 text-white"
                    }
                  >
                    {item.status}
                  </Badge>
                </CardContent>
              </Card>
            );
          })}
        </div>
      </section>

      <section>
        <h2 className="mb-4 text-2xl font-bold">路線一覧</h2>

        <div className="grid gap-4 md:grid-cols-2">
          {routes.data?.map((route) => (
            <Link key={route.id} to={`/routes/${route.id}`}>
              <Card className="transition-colors hover:bg-accent">
                <CardContent className="py-6">{route.name}</CardContent>
              </Card>
            </Link>
          ))}
        </div>
      </section>
    </div>
  );
}
