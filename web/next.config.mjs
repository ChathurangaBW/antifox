/** @type {import('next').NextConfig} */

// Static Export:`NEXT_EXPORT=1 next build` Output Pure Static Directory To web/out,You can throw it right in.
// nginx web Root directory runs. Development(next dev)Do not set the variable, keep /api Inverse and heat updates.
const isExport = process.env.NEXT_EXPORT === "1";
// Vercel demo:Let's go. mock,No backend, no need. /api Inverse.
const isMock = process.env.NEXT_PUBLIC_MOCK === "1";

const nextConfig = {
  reactCompiler: true,
  // Allow from LAN IP Visits dev ResourcesHMR),Add or delete as required.
  // dev Release any stage IPv4 Source visits /_next/* and HMR(LAN IP The change is also not affected.
  // Note: Next.js blocks a bare "*" for safety, so a pattern is required; "*.*.*.*" matches any IPv4 address.
  allowedDevOrigins: ["*.*.*.*"],
  compiler: {
    removeConsole: process.env.NODE_ENV === "production",
  },
  ...(isExport
    ? {
        // Pure static export: no Node runtime, images are not optimized, and each route emits <route>/index.html.
        output: "export",
        images: { unoptimized: true },
        trailingSlash: true,
      }
    : isMock
      ? {
          // Vercel mock demo:No back-end, no need. /api Inverse.
          images: { unoptimized: true },
        }
      : {
          // Development: /api/* Inversely. Go Backend (default) :8787,Available AUTOPENTEST_API Cover.
          async rewrites() {
            const backend = process.env.AUTOPENTEST_API ?? "http://localhost:8787";
            return [{ source: "/api/:path*", destination: `${backend}/api/:path*` }];
          },
        }),
};

export default nextConfig;
