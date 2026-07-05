import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://charliezo2001.github.io',
  base: '/study-backend-scale',
  integrations: [
    starlight({
      title: 'URL短縮で学ぶバックエンドスケーラビリティ',
      description: 'URL短縮サービスを題材に、Go、Docker、Kubernetes、計測、キャッシュ、スケール設計を15日で学ぶ教材です。',
      locales: {
        root: {
          label: '日本語',
          lang: 'ja',
        },
      },
      sidebar: [{ autogenerate: { directory: '.' } }],
    }),
  ],
});
