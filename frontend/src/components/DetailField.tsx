// DetailField renders a labeled read-only value in a definition list.
export function DetailField({ label, value }: { label: string; value?: string | number | null }) {
  const display = value === undefined || value === null || value === "" ? "—" : String(value);
  return (
    <div>
      <dt className="text-xs font-medium uppercase tracking-wide text-gray-500">{label}</dt>
      <dd className="mt-0.5 text-sm text-gray-800">{display}</dd>
    </div>
  );
}
