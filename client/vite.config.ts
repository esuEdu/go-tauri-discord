import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";

function woff2Only(): Plugin {
  return {
    name: "phosphor-woff2-only",
    enforce: "pre",
    transform(code, id) {
      if (!id.includes("@phosphor-icons/web")) return null;
      return code.replace(/src:[\s\S]*?;/, (block) => {
        const woff2 = block.match(/url\("([^"]+\.woff2)"\)/);
        return woff2 ? `src: url("${woff2[1]}") format("woff2");` : block;
      });
    },
  };
}

export default defineConfig({
  plugins: [woff2Only(), react()],
  clearScreen: false,
  server: {
    port: 1420,
    strictPort: true,
    proxy: {
      "/api": "http://localhost:8080",
      "/gateway": { target: "ws://localhost:8080", ws: true },
    },
  },
});
