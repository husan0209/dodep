const nextConfig = {
  reactStrictMode: true,
  swcMinify: true,
  output: "standalone",

  // Environment variables
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080",
    NEXT_PUBLIC_AUTH_API_URL: process.env.NEXT_PUBLIC_AUTH_API_URL || "http://localhost:8083",
    NEXT_PUBLIC_WS_URL: process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:8080",
    NEXT_PUBLIC_APP_ENV: process.env.NEXT_PUBLIC_APP_ENV || "development",
  },

  // Security headers
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          {
            key: "X-DNS-Prefetch-Control",
            value: "on",
          },
          {
            key: "Strict-Transport-Security",
            value: "max-age=63072000; includeSubDomains; preload",
          },
          {
            key: "X-Frame-Options",
            value: "DENY",
          },
          {
            key: "X-Content-Type-Options",
            value: "nosniff",
          },
          {
            key: "X-XSS-Protection",
            value: "1; mode=block",
          },
          {
            key: "Referrer-Policy",
            value: "strict-origin-when-cross-origin",
          },
          {
            key: "Permissions-Policy",
            value: "camera=(), microphone=(), geolocation=(self)",
          },
          {
            key: "Content-Security-Policy",
            value: [
              "default-src 'self'",
              "script-src 'self' 'unsafe-inline' 'unsafe-eval'",
              "style-src 'self' 'unsafe-inline'",
              "img-src 'self' data: https: blob:",
              "connect-src 'self' https://api.dod.casino wss://ws.dod.casino http://localhost:* ws://localhost:*",
              "frame-src https://*.casino-provider.com",
              "font-src 'self' data:",
            ].join("; "),
          },
        ],
      },
    ];
  },

  // Redirects
  async redirects() {
    return [
      {
        source: "/dashboard",
        destination: "/sportsbook",
        permanent: true,
      },
      {
        source: "/betting",
        destination: "/sportsbook",
        permanent: true,
      },
    ];
  },

  // Image optimization
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "cdn.dod.casino",
      },
      {
        protocol: "https",
        hostname: "images.dod.casino",
      },
    ],
    formats: ["image/avif", "image/webp"],
  },
};

export default nextConfig;
