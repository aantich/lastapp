import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: "/console/",
  build: { emptyOutDir: false },
  server: {
    host: "127.0.0.1",
    port: 5173,
    proxy: {
      "/auth": "http://127.0.0.1:8787",
      "/v1": "http://127.0.0.1:8787",
      "/admin/v1": "http://127.0.0.1:8787",
    },
  },
});
