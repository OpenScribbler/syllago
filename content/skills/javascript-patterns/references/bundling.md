# JavaScript Bundling

Configuration patterns for Vite, esbuild, and webpack.

## Tool Selection

| Tool | Best For |
|------|----------|
| Vite | Modern dev experience, fast HMR, React/Vue/Svelte |
| esbuild | Fastest builds, simple config, Node.js bundles |
| webpack | Complex apps, legacy support, extensive plugin ecosystem |

## Vite
**Severity**: medium | **Category**: tooling

- Rule: Use `defineConfig` from `vite`. Add framework plugin (e.g., `@vitejs/plugin-react`).
- Rule: Configure `server.proxy` for API proxying during development.
- Rule: Use `build.rollupOptions.output.manualChunks` to split vendor bundles.

```javascript
// vite.config.js
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: { '/api': { target: 'http://localhost:8080', changeOrigin: true } },
  },
  build: {
    sourcemap: true,
    rollupOptions: {
      output: { manualChunks: { vendor: ['react', 'react-dom'] } },
    },
  },
});
```

- Rule: Use `path.resolve(__dirname, './src')` in `resolve.alias` for path aliases.
- Rule: Use `defineConfig(({ command, mode }) => ...)` for environment-specific config.

## esbuild
**Severity**: medium | **Category**: tooling

- Rule: Set `platform: 'browser'` for web, `platform: 'node'` for Node.js bundles.
- Rule: Use `external` array for native modules that should not be bundled (e.g., `pg-native`).
- Rule: Use `esbuild.context()` for watch mode with `ctx.watch()` and optional `ctx.serve()`.
- Gotcha: esbuild does not perform type checking -- run `tsc --noEmit` separately.

## webpack
**Severity**: medium | **Category**: tooling

- Rule: Use `[contenthash]` in output filenames for cache busting.
- Rule: Configure `optimization.splitChunks.chunks: 'all'` to split vendor code automatically.
- Rule: Use `TerserPlugin` with `drop_console: true` and `CssMinimizerPlugin` for production builds.
- Rule: Set `resolve.extensions` to `['.js', '.jsx', '.ts', '.tsx']` and `resolve.alias` for path mapping.
- Gotcha: `HtmlWebpackPlugin` must be configured to inject bundles into the template.

## TypeScript with Bundlers
**Severity**: medium | **Category**: setup

- Rule: For bundler-based projects, use `"moduleResolution": "bundler"`, `"noEmit": true`, and `"isolatedModules": true` in tsconfig.
- Rule: Add `"jsx": "react-jsx"` for React projects (no need to import React in every file).

## Bundle Analysis

- Rule: Use `npx vite-bundle-visualizer` (Vite) or `webpack-bundle-analyzer` plugin (webpack) to identify bloat.

For bundling anti-patterns, see `anti-patterns.md`.
