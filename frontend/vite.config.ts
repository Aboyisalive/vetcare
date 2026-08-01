import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
// Both proxies stand in for the /api/* rewrite the deployed static site does
// (see render.yaml), so the browser only ever sees one origin and the session
// cookie stays first-party.
const apiProxy = { '/api': 'http://localhost:8080' }

export default defineConfig({
  plugins: [react()],
  server: { proxy: apiProxy },
  // `npm run preview` serves the real production bundle, so it is the closest
  // local mirror of the Render deploy.
  preview: { proxy: apiProxy },
})
