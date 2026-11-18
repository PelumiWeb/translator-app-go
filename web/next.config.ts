import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // A static site: `next build` writes plain HTML, CSS and JS into out/, which
  // any web server can serve. Nothing on it needs a server (ADR 0028).
  output: "export",
  // The project is this folder, whatever lock files sit in the folders above.
  turbopack: { root: __dirname },
};

export default nextConfig;
