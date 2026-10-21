import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

const proxyTarget = process.env.VITE_PROXY_TARGET || "http://127.0.0.1:8080";

export default defineConfig({
  plugins: [react()],
  esbuild: {
    target: "es2022",
  },
  optimizeDeps: {
    esbuildOptions: { target: "es2022" },
    exclude: ["@novnc/novnc"],
  },
  build: {
    target: "es2022",
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  server: {
    host: "0.0.0.0",
    port: 5173,
    proxy: {
      "/api": { target: proxyTarget, changeOrigin: true, ws: true },
      "/healthz": { target: proxyTarget, changeOrigin: true },
    },
  },
});
