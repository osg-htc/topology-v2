"use client";

import { Fragment, ReactNode, useState } from "react";
import Link from "next/link";
import { DeleteButton } from "./entityActions";

export type Column<T> = {
  header: string;
  cell: (row: T) => ReactNode;
  className?: string;
  // sortValue enables click-to-sort on this column. Omit to make it unsortable.
  sortValue?: (row: T) => string | number | boolean | null | undefined;
};

// RowActions describes the pop-out (standalone page), edit (form), and delete
// affordances shown on the right of each row and in the expanded card.
export type RowActions = {
  detailHref: string; // standalone detail page ("pop-out")
  editHref: string; // edit form
  entityKind: string; // for the delete change request
  name: string; // target_name sent with the delete change request
  displayName?: string; // shown in the confirm dialog/aria-label; defaults to name
  onChanged?: () => void;
};

function PopOutIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="h-4 w-4">
      <path d="M14 4h6v6M20 4l-8 8M18 14v5a1 1 0 01-1 1H5a1 1 0 01-1-1V7a1 1 0 011-1h5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
function EditIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="h-4 w-4">
      <path d="M12 20h9M16.5 3.5a2.1 2.1 0 013 3L7 19l-4 1 1-4 12.5-12.5z" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function Actions({ a, canDelete }: { a: RowActions; canDelete: boolean }) {
  return (
    <div className="flex items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
      <Link href={a.editHref} title="Edit" aria-label={`Edit ${a.name}`} className="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-brand-700">
        <EditIcon />
      </Link>
      <Link href={a.detailHref} title="Open standalone page" aria-label={`Open ${a.name}`} className="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-brand-700">
        <PopOutIcon />
      </Link>
      {canDelete && (
        <DeleteButton
          entityKind={a.entityKind}
          name={a.name}
          displayName={a.displayName}
          onDone={a.onChanged}
        />
      )}
    </div>
  );
}

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  expanded,
  actions,
  canDelete,
}: {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  expanded?: (row: T) => ReactNode;
  actions?: (row: T) => RowActions;
  canDelete?: boolean;
}) {
  const [open, setOpen] = useState<string | null>(null);
  const [sort, setSort] = useState<{ col: number; dir: 1 | -1 } | null>(null);
  const colSpan = columns.length + (actions ? 1 : 0);

  const toggleSort = (i: number) => {
    if (!columns[i].sortValue) return;
    setSort((s) => (s && s.col === i ? { col: i, dir: (s.dir * -1) as 1 | -1 } : { col: i, dir: 1 }));
  };

  const sortedRows = sort
    ? [...rows].sort((a, b) => {
        const sv = columns[sort.col].sortValue!;
        const va = sv(a);
        const vb = sv(b);
        const na = va == null ? "" : va;
        const nb = vb == null ? "" : vb;
        if (typeof na === "number" && typeof nb === "number") return (na - nb) * sort.dir;
        return String(na).localeCompare(String(nb), undefined, { numeric: true }) * sort.dir;
      })
    : rows;

  return (
    <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
      <table className="min-w-full text-sm">
        <thead className="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
          <tr>
            {columns.map((c, i) => (
              <th
                key={c.header}
                className={`px-4 py-2 ${c.className ?? ""} ${c.sortValue ? "cursor-pointer select-none hover:text-gray-800" : ""}`}
                onClick={() => toggleSort(i)}
              >
                {c.header}
                {c.sortValue && (
                  <span className="ml-1 text-gray-400">
                    {sort && sort.col === i ? (sort.dir === 1 ? "▲" : "▼") : "⇅"}
                  </span>
                )}
              </th>
            ))}
            {actions && <th className="px-4 py-2" />}
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100">
          {sortedRows.map((row) => {
            const key = rowKey(row);
            const isOpen = open === key;
            return (
              <Fragment key={key}>
                <tr
                  className={`cursor-pointer hover:bg-gray-50 ${isOpen ? "bg-gray-50" : ""}`}
                  onClick={() => expanded && setOpen(isOpen ? null : key)}
                >
                  {columns.map((c, i) => (
                    <td key={i} className={`px-4 py-2 ${c.className ?? ""}`}>
                      {c.cell(row)}
                    </td>
                  ))}
                  {actions && (
                    <td className="px-4 py-2 text-right">
                      <Actions a={actions(row)} canDelete={!!canDelete} />
                    </td>
                  )}
                </tr>
                {expanded && isOpen && (
                  <tr>
                    <td colSpan={colSpan} className="bg-gray-50/60 px-4 py-4">
                      {expanded(row)}
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
