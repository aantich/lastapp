import { defineConfig } from "vite";
import { readFileSync } from "node:fs";
export default defineConfig({
  base: "./",
  server: { host: "127.0.0.1", port: 5173, strictPort: true },
  plugins: [
    {
      name: "lastapp-local-config",
      configureServer(server) {
        server.middlewares.use("/_lastapp/config.json", (_req, res) => {
          res.setHeader("Content-Type", "application/json");
          res.setHeader("Cache-Control", "no-store");
          res.end(readFileSync("lastapp.local.json", "utf8"));
        });
      },
    },
  ],
});
