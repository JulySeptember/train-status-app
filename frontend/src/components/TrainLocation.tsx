import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

import { Badge } from "@/components/ui/badge";

import { type TrainLocation as Train } from "@/types";

type Props = {
  train: Train;
};

export default function TrainLocation({ train }: Props) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>列車位置</CardTitle>
      </CardHeader>

      <CardContent className="space-y-3">
        <p>
          <strong>路線：</strong>

          {train.railway}
        </p>

        <p>
          <strong>現在位置：</strong>

          {train.fromStation}

          {" → "}

          {train.toStation}
        </p>

        <div>
          <Badge>遅延 {train.delay} 秒</Badge>
        </div>
      </CardContent>
    </Card>
  );
}
