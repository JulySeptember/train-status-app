import { type ReactNode } from "react";
import {
  DndContext,
  KeyboardSensor,
  MouseSensor,
  TouchSensor,
  closestCenter,
  useSensor,
  useSensors,
  type Announcements,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  rectSortingStrategy,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { GripVertical } from "lucide-react";

type Props<T> = {
  items: T[];
  getId: (item: T) => string;
  getLabel: (item: T) => string;
  // 並び替えた結果を、動かした ID と移動先の ID で返す
  onMove: (activeId: string, overId: string) => void;
  renderItem: (item: T) => ReactNode;
  layout?: "list" | "grid";
  className?: string;
};

function SortableItem({
  id,
  label,
  children,
}: {
  id: string;
  label: string;
  children: ReactNode;
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    setActivatorNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id });

  return (
    <div
      ref={setNodeRef}
      style={{
        transform: CSS.Translate.toString(transform),
        transition,
      }}
      className={`flex items-stretch gap-2 ${
        isDragging ? "relative z-10 opacity-80" : ""
      }`}
    >
      {/*
        ドラッグはこのつまみからだけ始める。カードを触ってスクロールしてもドラッグにならず、
        つまみはリンクの外にあるので、離したときに路線ページへ移動しない
      */}
      <button
        type="button"
        ref={setActivatorNodeRef}
        {...attributes}
        {...listeners}
        aria-label={`${label}を並び替える`}
        className={`flex w-8 shrink-0 touch-manipulation items-center justify-center rounded-md text-gray-500 transition hover:bg-[#21262d] hover:text-gray-300 focus-visible:outline-2 focus-visible:outline-[#58a6ff] ${
          isDragging ? "cursor-grabbing" : "cursor-grab"
        }`}
      >
        <GripVertical size={18} />
      </button>

      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

export default function SortableList<T>({
  items,
  getId,
  getLabel,
  onMove,
  renderItem,
  layout = "list",
  className,
}: Props<T>) {
  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 5 } }),
    // スマホでは長押ししてからドラッグを始め、スクロールと区別する
    useSensor(TouchSensor, {
      activationConstraint: { delay: 200, tolerance: 8 },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );

  const labelOf = (id: string | number) => {
    const item = items.find((i) => getId(i) === id);
    return item ? getLabel(item) : String(id);
  };

  const announcements: Announcements = {
    onDragStart: ({ active }) => `${labelOf(active.id)}を持ち上げました。`,
    onDragOver: ({ active, over }) =>
      over
        ? `${labelOf(active.id)}を${labelOf(over.id)}の位置に動かしました。`
        : undefined,
    onDragEnd: ({ active, over }) =>
      over
        ? `${labelOf(active.id)}を${labelOf(over.id)}の位置に置きました。`
        : `${labelOf(active.id)}を置きました。`,
    onDragCancel: ({ active }) =>
      `並び替えを取り消しました。${labelOf(active.id)}は元の位置に戻りました。`,
  };

  const handleDragEnd = ({ active, over }: DragEndEvent) => {
    if (over && active.id !== over.id) {
      onMove(String(active.id), String(over.id));
    }
  };

  const ids = items.map(getId);

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragEnd={handleDragEnd}
      accessibility={{
        announcements,
        screenReaderInstructions: {
          draggable:
            "スペースキーで持ち上げ、矢印キーで動かし、スペースキーで置きます。Escape キーで取り消します。",
        },
      }}
    >
      <SortableContext
        items={ids}
        strategy={
          layout === "grid" ? rectSortingStrategy : verticalListSortingStrategy
        }
      >
        <div className={className}>
          {items.map((item) => (
            <SortableItem
              key={getId(item)}
              id={getId(item)}
              label={getLabel(item)}
            >
              {renderItem(item)}
            </SortableItem>
          ))}
        </div>
      </SortableContext>
    </DndContext>
  );
}
