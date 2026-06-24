import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// 개발 중 /api 요청을 로컬 백엔드(기본 8080)로 프록시.
// 운영에서는 nginx/Ingress가 /api를 백엔드 서비스로 라우팅한다.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: process.env.VITE_API_PROXY_TARGET || "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
