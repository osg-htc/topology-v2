"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";
import { MultiSelect } from "@/components/MultiSelect";

const CLASSES = ["SCHEDULED", "UNSCHEDULED"];
const SEVERITIES = ["Outage", "Severe", "Intermittent Outage", "No Significant Outage Expected"];

// Convert a stored canonical downtime timestamp (or anything Date can parse)
// into the value a datetime-local input expects (UTC, minute precision).
function toLocalInput(s: string): string {
  if (!s) return "";
  const d = new Date(s);
  if (isNaN(d.getTime())) return "";
  return d.toISOString().slice(0, 16);
}

function DowntimeFormView() {
  const router = useRouter();
  const params = useSearchParams();
  const editId = params.get("edit"); // dt_id for update
  const presetResource = params.get("resource") ?? "";

  const { data: resourceMap } = useQuery({
    queryKey: ["resources-map"],
    queryFn: () => api.resources(),
  });
  const { data: serviceNames } = useQuery({
    queryKey: ["service-names"],
    queryFn: () => api.serviceNames(),
  });
  // For edit, load the existing downtime so we can prefill.
  const { data: existing } = useQuery({
    queryKey: ["downtime-for-edit", editId],
    queryFn: () => api.downtimes(),
    enabled: !!editId,
  });

  const resourceNames = useMemo(
    () =>
      Object.values(resourceMap ?? {})
        .map((r) => r.name)
        .sort((a, b) => a.localeCompare(b)),
    [resourceMap],
  );

  const [resource, setResource] = useState(presetResource);
  const [cls, setCls] = useState("SCHEDULED");
  const [severity, setSeverity] = useState(SEVERITIES[0]);
  const [description, setDescription] = useState("");
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [services, setServices] = useState<string[]>([]);
  const [showErrors, setShowErrors] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [loadedEdit, setLoadedEdit] = useState(false);

  // Prefill once the edit target is found in the list.
  useEffect(() => {
    if (!editId || loadedEdit || !existing) return;
    const d = existing.find((x) => String(x.id) === editId);
    if (!d) return;
    setResource(d.resource);
    setCls(d.class || "SCHEDULED");
    setSeverity(d.severity || SEVERITIES[0]);
    setDescription(d.description || "");
    setStart(toLocalInput(d.start_time));
    setEnd(toLocalInput(d.end_time));
    setServices(d.services ?? []);
    setLoadedEdit(true);
  }, [editId, existing, loadedEdit]);

  const invalid = {
    resource: !resource,
    start: !start,
    end: !end || (!!start && !!end && end <= start),
  };
  const errCls = (bad: boolean) =>
    showErrors && bad ? "ring-2 ring-red-400 border-red-400" : "";

  const submit = async (draft: boolean) => {
    setErr("");
    if (invalid.resource || invalid.start || invalid.end) {
      setShowErrors(true);
      setErr("Please fix the highlighted fields (end must be after start).");
      return;
    }
    setBusy(true);
    try {
      const res = await api.proposals.create({
        entity_kind: "downtime",
        operation: editId ? "update" : "create",
        target_name: editId ?? undefined,
        submit: !draft,
        proposed_state: {
          resource,
          class: cls,
          severity,
          description,
          start_time: `${start}:00Z`,
          end_time: `${end}:00Z`,
          services,
        },
      });
      router.push(`/proposals/view?id=${res.id}`);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="p-8">
      <PageHeader
        title={editId ? "Edit downtime" : "Register a downtime"}
        description="Downtimes are applied through the change-request workflow."
      />
      <Card className="max-w-2xl space-y-4">
        <div>
          <label className={label}>Resource</label>
          <input
            className={`${input} ${errCls(invalid.resource)}`}
            list="dt-resources"
            value={resource}
            disabled={!!editId}
            onChange={(e) => setResource(e.target.value)}
            placeholder="Select a resource…"
          />
          <datalist id="dt-resources">
            {resourceNames.map((n) => (
              <option key={n} value={n} />
            ))}
          </datalist>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <label className={label}>Class</label>
            <select className={input} value={cls} onChange={(e) => setCls(e.target.value)}>
              {CLASSES.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={label}>Severity</label>
            <select
              className={input}
              value={severity}
              onChange={(e) => setSeverity(e.target.value)}
            >
              {SEVERITIES.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <label className={label}>Start (UTC)</label>
            <input
              type="datetime-local"
              className={`${input} ${errCls(invalid.start)}`}
              value={start}
              onChange={(e) => setStart(e.target.value)}
            />
          </div>
          <div>
            <label className={label}>End (UTC)</label>
            <input
              type="datetime-local"
              className={`${input} ${errCls(invalid.end)}`}
              value={end}
              onChange={(e) => setEnd(e.target.value)}
            />
          </div>
        </div>

        <div>
          <label className={label}>Affected services</label>
          <MultiSelect
            options={serviceNames ?? []}
            value={services}
            onChange={setServices}
            placeholder="Add a service…"
            allowCustom={false}
          />
        </div>

        <div>
          <label className={label}>Description</label>
          <textarea
            className={input}
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Reason for the downtime…"
          />
        </div>

        {err && <p className="text-sm text-red-600">{err}</p>}

        <div className="flex gap-3">
          <button className={btn} disabled={busy} onClick={() => submit(false)}>
            {editId ? "Submit change" : "Submit for review"}
          </button>
          <button className={btnSecondary} disabled={busy} onClick={() => submit(true)}>
            Save as draft
          </button>
        </div>
      </Card>
    </div>
  );
}

export default function DowntimeFormPage() {
  return (
    <Suspense fallback={null}>
      <DowntimeFormView />
    </Suspense>
  );
}
