import path from "node:path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

// Em desenvolvimento, as chamadas da API vão para o servidor Go (REQUERIAL_API, padrão 127.0.0.1:8787).
const api = process.env.REQUERIAL_API ?? "http://127.0.0.1:8787"
const rotasDaApi = ["/tarefas", "/sessoes", "/eventos"]

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { "@": path.resolve(__dirname, "./src") } },
  server: { proxy: Object.fromEntries(rotasDaApi.map((r) => [r, api])) },
})
