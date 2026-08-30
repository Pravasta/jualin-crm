import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // API key management moved from /settings/api-keys to /connect/api
  // (issue #86/ADR-012) — the old URLs stay alive as permanent (308)
  // redirects, not deleted, so anything bookmarked or linked
  // externally keeps working. Acceptance criterion: verified by
  // opening the old URL directly, not by reading this file.
  async redirects() {
    return [
      {
        source: "/settings/api-keys",
        destination: "/connect/api",
        permanent: true,
      },
      {
        source: "/settings/api-keys/docs",
        destination: "/connect/api/docs",
        permanent: true,
      },
    ];
  },
};

export default nextConfig;
