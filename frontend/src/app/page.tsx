export default function HomePage() {
  return (
    <main className="mx-auto max-w-3xl px-6 py-16">
      <h1 className="text-3xl font-bold text-navy-900">OSG Topology</h1>
      <p className="mt-4 text-gray-600">
        Resource registration and management for the OSG. The dashboard,
        resource-centric browsing, and change-proposal workflow are under
        construction.
      </p>
      <div className="mt-8 rounded-lg border border-gray-200 bg-white p-6">
        <p className="text-sm text-gray-500">
          Backend health:{" "}
          <a href="/healthz" className="text-brand-600 hover:underline">
            /healthz
          </a>
        </p>
      </div>
    </main>
  );
}
