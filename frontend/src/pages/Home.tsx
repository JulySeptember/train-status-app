import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { arrayMove } from "@dnd-kit/sortable";
import { RotateCcw } from "lucide-react";

import { api } from "@/api";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import SortableList from "@/components/SortableList";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { applyOrder, sortByOrder, useRailwayOrder } from "@/lib/railwayOrder";

export default function Home() {
  const status = useQuery({
    queryKey: ["status"],
    queryFn: api.getStatus,
  });

  const routes = useQuery({
    queryKey: ["routes"],
    queryFn: api.getRoutes,
  });

  const railwayOrder = useRailwayOrder();

  if (status.isPending || routes.isPending) {
    return <Loading />;
  }

  if (status.error || routes.error) {
    return <Error />;
  }

  // 運行情報と路線一覧は同じ並び順を使う。路線一覧の順を基準に、保存した並び順を当てはめる
  const order = applyOrder(
    [...routes.data.map((r) => r.id), ...status.data.map((s) => s.railwayId)],
    railwayOrder.order,
  );

  const statuses = sortByOrder(status.data, (s) => s.railwayId, order);
  const railways = sortByOrder(routes.data, (r) => r.id, order);

  // 一方の一覧で動かしても、全体の並び順の中で動かせば、もう一方の一覧も同じ順になる
  const move = (activeId: string, overId: string) => {
    railwayOrder.setOrder(
      arrayMove(order, order.indexOf(activeId), order.indexOf(overId)),
    );
  };

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm text-muted-foreground">
          左のつまみをドラッグすると、路線を並び替えられます。
        </p>

        {railwayOrder.order && (
          <Button variant="outline" size="sm" onClick={railwayOrder.reset}>
            <RotateCcw />
            並び順を戻す
          </Button>
        )}
      </div>

      <section>
        <h2 className="mb-4 text-2xl font-bold">運行情報</h2>

        <SortableList
          items={statuses}
          getId={(item) => item.railwayId}
          getLabel={(item) => item.railway}
          onMove={move}
          className="space-y-3"
          renderItem={(item) => {
            const isNormal =
              item.status.includes("現在、15分以上の遅延はありません") ||
              item.status.includes("現在、１５分以上の遅延はありません");

            return (
              <Card>
                <CardHeader>
                  <CardTitle>{item.railway}</CardTitle>
                </CardHeader>

                <CardContent>
                  <Badge
                    className={
                      isNormal
                        ? "border border-brand/40 bg-brand/15 text-foreground"
                        : "border border-destructive/40 bg-destructive/15 text-foreground"
                    }
                  >
                    {item.status}
                  </Badge>
                </CardContent>
              </Card>
            );
          }}
        />
      </section>

      <section>
        <h2 className="mb-4 text-2xl font-bold">路線一覧</h2>

        <SortableList
          items={railways}
          getId={(route) => route.id}
          getLabel={(route) => route.name}
          onMove={move}
          layout="grid"
          className="grid gap-4 md:grid-cols-2"
          renderItem={(route) => (
            <Link to={`/routes/${route.id}`}>
              <Card className="transition-colors hover:bg-accent">
                <CardContent className="py-6">{route.name}</CardContent>
              </Card>
            </Link>
          )}
        />
      </section>
    </div>
  );
}
