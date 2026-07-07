import { Link } from "react-router-dom";

import { type DirectionTimetable } from "@/types";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

type Props = {
  weekday?: DirectionTimetable;
  saturday?: DirectionTimetable;
  holiday?: DirectionTimetable;
  saturdayHoliday?: DirectionTimetable;
};

function TimetableTable({
  title,
  timetable,
}: {
  title: string;
  timetable?: DirectionTimetable;
}) {
  return (
    <div className="flex-1">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="text-center">{title}</TableHead>
          </TableRow>
        </TableHeader>

        <TableBody>
          {timetable?.timetables.map((train) => (
            <TableRow key={`${train.time}-${train.trainNumber}`}>
              <TableCell className="text-center">
                <Link to={`/trains/${train.trainNumber}`} className="block">
                  {train.time}
                </Link>
              </TableCell>
            </TableRow>
          ))}

          {!timetable && (
            <TableRow>
              <TableCell className="text-center">-</TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  );
}

export default function Timetable({
  weekday,
  saturday,
  holiday,
  saturdayHoliday,
}: Props) {
  if (saturdayHoliday) {
    return (
      <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
        <TimetableTable title="平日" timetable={weekday} />
        <TimetableTable title="土休日" timetable={saturdayHoliday} />
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
      <TimetableTable title="平日" timetable={weekday} />
      <TimetableTable title="土曜" timetable={saturday} />
      <TimetableTable title="休日" timetable={holiday} />
    </div>
  );
}
