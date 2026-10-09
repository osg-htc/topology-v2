"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, LinkButton } from "@/components/ui";
import { DetailField } from "@/components/DetailField";
import { EntityProposalHistory } from "@/components/EntityProposalHistory";

const asList = (v: unknown): string[] => (Array.isArray(v) ? v.filter((x): x is string => typeof x === "string") : []);

function VODetail() {
  const name = useSearchParams().get("name") || "";
  const { data, isLoading } = useQuery({ queryKey: ["vo", name], queryFn: () => api.vo(name), enabled: !!name });

  if (isLoading) return <div className="p-8 text-gray-400">Loading…</div>;
  if (!data) return <div className="p-8 text-gray-400">VO not found.</div>;
  const vo = data.vo;
  const contacts = Object.entries(vo.Contacts ?? {});
  const primary = asList(vo.FieldsOfScience?.PrimaryFields);
  const secondary = asList(vo.FieldsOfScience?.SecondaryFields);
  const groups = asList(vo.ReportingGroups);
  const issuers = ((vo.Credentials as { TokenIssuers?: { URL?: string; DefaultUnixUser?: string }[] } | null)?.TokenIssuers ?? []);
  const link = (kind: string, n: string) => (
    <Link key={n} href={`/${kind}/detail?name=${encodeURIComponent(n)}`} className="text-brand-700 hover:underline">{n}</Link>
  );
  const list = (items: string[], render: (n: string) => React.ReactNode) =>
    items.length === 0 ? <li className="text-gray-400">None.</li> : items.map((n) => <li key={n}>{render(n)}</li>);

  return (
    <div className="p-8">
      <PageHeader
        title={data.name}
        description="Virtual organization"
        action={
          <div className="flex items-center gap-2">
            {data.disable && <span className="rounded-full bg-red-100 px-3 py-1 text-xs font-medium text-red-700">disabled</span>}
            {vo.Active === false && <span className="rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-500">inactive</span>}
            <LinkButton href={`/vos/new?edit=${encodeURIComponent(data.name)}`}>Edit</LinkButton>
          </div>
        }
      />
      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <Card>
            <dl className="grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2">
              <DetailField label="ID" value={data.id} />
              <DetailField label="Long name" value={vo.LongName ?? ""} />
              <DetailField label="Community" value={vo.Community ?? ""} />
              <DetailField label="Certificate only" value={vo.CertificateOnly ? "yes" : "no"} />
              <DetailField label="Primary URL" value={vo.PrimaryURL ?? ""} />
              <DetailField label="Purpose URL" value={vo.PurposeURL ?? ""} />
              <DetailField label="Support URL" value={vo.SupportURL ?? ""} />
              <DetailField label="Membership services URL" value={vo.MembershipServicesURL ?? ""} />
            </dl>
            {vo.AppDescription && (
              <div className="mt-4">
                <div className="text-xs font-medium uppercase tracking-wide text-gray-500">Application description</div>
                <p className="mt-1 whitespace-pre-wrap text-sm text-gray-700">{vo.AppDescription}</p>
              </div>
            )}
          </Card>
          {(primary.length > 0 || secondary.length > 0) && (
            <Card>
              <h3 className="mb-2 text-sm font-semibold text-gray-700">Fields of science</h3>
              <dl className="space-y-2 text-sm">
                {primary.length > 0 && (<div><dt className="text-xs font-medium uppercase tracking-wide text-gray-500">Primary</dt><dd className="text-gray-700">{primary.join(", ")}</dd></div>)}
                {secondary.length > 0 && (<div><dt className="text-xs font-medium uppercase tracking-wide text-gray-500">Secondary</dt><dd className="text-gray-700">{secondary.join(", ")}</dd></div>)}
              </dl>
            </Card>
          )}
          {contacts.length > 0 && (
            <Card>
              <h3 className="mb-2 text-sm font-semibold text-gray-700">Contacts</h3>
              <dl className="space-y-2 text-sm">
                {contacts.map(([type, people]) => (
                  <div key={type}>
                    <dt className="text-xs font-medium uppercase tracking-wide text-gray-500">{type}</dt>
                    <dd className="text-gray-700">{(people ?? []).map((p) => p.Name).filter(Boolean).join(", ") || "—"}</dd>
                  </div>
                ))}
              </dl>
            </Card>
          )}
          {(vo.OASIS || issuers.length > 0) && (
            <Card>
              <h3 className="mb-2 text-sm font-semibold text-gray-700">OASIS &amp; credentials</h3>
              <dl className="grid grid-cols-1 gap-x-8 gap-y-3 text-sm sm:grid-cols-2">
                {vo.OASIS && <DetailField label="Uses OASIS" value={(vo.OASIS as { UseOASIS?: boolean }).UseOASIS ? "yes" : "no"} />}
                {issuers.length > 0 && <DetailField label="Token issuers" value={issuers.map((i) => i.URL).filter(Boolean).join(", ")} />}
              </dl>
              <p className="mt-3 text-xs text-gray-400">OASIS, credentials and data-federation settings are kept as-is when a VO is edited with the form; change them with the form&apos;s advanced editor.</p>
            </Card>
          )}
        </div>
        <div className="space-y-6">
          <Card>
            <h3 className="mb-2 text-sm font-semibold text-gray-700">Relationships</h3>
            <dl className="space-y-3 text-sm">
              <div><dt className="text-xs font-medium uppercase tracking-wide text-gray-500">Parent VO</dt><dd>{vo.ParentVO?.Name ? link("vos", vo.ParentVO.Name) : <span className="text-gray-400">None</span>}</dd></div>
              <div><dt className="text-xs font-medium uppercase tracking-wide text-gray-500">Child VOs ({data.child_vos.length})</dt><dd><ul className="space-y-1">{list(data.child_vos, (n) => link("vos", n))}</ul></dd></div>
              <div><dt className="text-xs font-medium uppercase tracking-wide text-gray-500">Reporting groups</dt><dd className="text-gray-700">{groups.length ? groups.join(", ") : "None"}</dd></div>
            </dl>
          </Card>
          <Card>
            <h3 className="mb-2 text-sm font-semibold text-gray-700">In use by</h3>
            <dl className="space-y-3 text-sm">
              <div><dt className="text-xs font-medium uppercase tracking-wide text-gray-500">Resources that allow it ({data.resources.length})</dt><dd className="text-gray-700">{data.resources.slice(0, 8).join(", ") || "None"}{data.resources.length > 8 && ` … +${data.resources.length - 8} more`}</dd></div>
              <div><dt className="text-xs font-medium uppercase tracking-wide text-gray-500">Projects it sponsors ({data.projects.length})</dt><dd className="text-gray-700">{data.projects.slice(0, 8).join(", ") || "None"}{data.projects.length > 8 && ` … +${data.projects.length - 8} more`}</dd></div>
            </dl>
          </Card>
          <EntityProposalHistory entityKind="vo" targetName={data.name} />
        </div>
      </div>
    </div>
  );
}

export default function VODetailPage() {
  return (
    <Suspense fallback={null}>
      <VODetail />
    </Suspense>
  );
}
