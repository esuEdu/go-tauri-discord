import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  clearScreen: false,
  server: {
    port: 1420,
    strictPort: true,
    // The app talks to its own origin, so dev proxies to the Go server and
    // production is served by it directly. Same code path either way, and no
    // CORS in the picture at all.
    proxy: {
      "/api": "http://localhost:8080",
      "/gateway": { target: "ws://localhost:8080", ws: true },
    },
  },
});
