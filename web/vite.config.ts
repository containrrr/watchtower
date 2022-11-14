import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

const htmlPlugin = (mode: string) => {
  const env = loadEnv(mode, ".");

  return {
    name: "html-transform",
    transformIndexHtml(html: string) {
      return html.replace(/%(.*?)%/g, function (match, p1) {
        return env[p1];
      });
    },
  };
};

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => ({
  plugins: [react(), htmlPlugin(mode)]
}));
