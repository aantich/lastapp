import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
// The API the dev server proxies to; scripts/e2e.py points it at the e2e API.
const api = process.env.LASTAPP_API_ORIGIN || "http://127.0.0.1:8787";
export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: "/console/",
  build: { emptyOutDir: false },
  server: {
    host: "127.0.0.1",
    port: 5173,
    proxy: {
      "/auth": api,
      "/v1": api,
      "/admin/v1": api,
    },
  },
});
