import { type Passenger } from "@/types";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

type Props = {
  passengers: Passenger[];
};

export default function PassengerTable({ passengers }: Props) {
  if (!passengers || passengers.length === 0) {
    return null;
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>年度</TableHead>

          <TableHead>乗降者数</TableHead>
        </TableRow>
      </TableHeader>

      <TableBody>
        {passengers.map((row) => (
          <TableRow key={row.year}>
            <TableCell>{row.year}</TableCell>

            <TableCell>{row.count.toLocaleString()} 人</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
