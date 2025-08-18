import react from '@vitejs/plugin-react-swc'
import { defineConfig } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'
// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'prompt',
      includeAssets: [
        'favicon.ico',
        'robots.txt',
        'apple-touch-icon.png',
        'safari-pinned-tab.svg',
        'mstile-150x150.png',
      ],
      injectManifest: {
        globPatterns: ['**/*.{js,css,html,png,svg}'],
        globIgnores: ['index.html'],
      },
      manifest: {
        name: 'Donetick: Simplify Tasks & Chores, Together.',
        short_name: 'Donetick',
        icons: [
          {
            src: '/android-chrome-192x192.png',
            sizes: '192x192',
            type: 'image/png',
          },
          {
            src: '/android-chrome-512x512.png',
            sizes: '512x512',
            type: 'image/png',
          },
          {
            src: 'pwa-64x64.png',
            sizes: '64x64',
            type: 'image/png',
          },
          {
            src: 'pwa-192x192.png',
            sizes: '192x192',
            type: 'image/png',
          },
          {
            src: 'pwa-512x512.png',
            sizes: '512x512',
            type: 'image/png',
          },
          {
            src: 'maskable-icon-512x512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
        ],
        theme_color: '#ffffff',
        background_color: '#ffffff',
        display: 'standalone',
      },
      workbox: {
        skipWaiting: true, // Force the waiting service worker to become the active service worker
        clientsClaim: true, // Take control of uncontrolled clients as soon as the service worker becomes active
        maximumFileSizeToCacheInBytes: 6000000, // 6MB
      },
    }),
  ],

  resolve: {
    alias: [
      {
        find: '@',
        replacement: '/src',
      },
    ],
  },
})
