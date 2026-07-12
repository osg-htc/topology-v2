/** @type {import('next').NextConfig} */
const isProd = process.env.NODE_ENV === "production";

const nextConfig = {
  // Production: static export → embedded in the Go binary via go:embed.
  ...(isProd ? { output: "export" } : {}),
  images: { unoptimized: true },
  async rewrites() {
    if (isProd) return [];
    // Dev: proxy API/auth traffic to the Go backend on :8080.
    return [
      { source: "/api/:path*", destination: "http://localhost:8080/api/:path*" },
      { source: "/healthz", destination: "http://localhost:8080/healthz" },
    ];
  },
};

export default nextConfig;
