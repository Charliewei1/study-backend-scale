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
      sidebar: [
        {
          label: 'はじめに',
          items: [{ label: 'コース概要', slug: 'index' }],
        },
        {
          label: '基礎(day01-04)',
          items: [
            { label: 'Day 1: Go 入門と最小の URL 短縮サーバ', slug: 'day01' },
            { label: 'Day 2: API 設計とテスト文化', slug: 'day02' },
            { label: 'Day 3: 永続化とインターフェース設計', slug: 'day03' },
            { label: 'Day 4: Go の並行処理とクリック統計', slug: 'day04' },
          ],
        },
        {
          label: 'コンテナ(day05-06)',
          items: [
            { label: 'Day 5: Docker でコンテナ化', slug: 'day05' },
            { label: 'Day 6: Docker Compose と Postgres 移行', slug: 'day06' },
          ],
        },
        {
          label: '計測とキャッシュ(day07-08)',
          items: [
            { label: 'Day 7: 計測: k6 ベースラインと pprof', slug: 'day07' },
            { label: 'Day 8: Redis キャッシュ: 読み取りを速くする', slug: 'day08' },
          ],
        },
        {
          label: 'Kubernetes(day09-11)',
          items: [
            { label: 'Day 9: Kubernetes 入門: kind で動かす', slug: 'day09' },
            { label: 'Day 10: k8s 設定・ヘルスチェック・ローリング更新', slug: 'day10' },
            { label: 'Day 11: 水平スケールと HPA', slug: 'day11' },
          ],
        },
        {
          label: '運用と分散(day12-14)',
          items: [
            { label: 'Day 12: 可観測性: Prometheus と structured logging', slug: 'day12' },
            { label: 'Day 13: レートリミットと回復性: 入口で守る', slug: 'day13' },
            { label: 'Day 14: さらなるスケール — シャーディングと分散の理論', slug: 'day14' },
          ],
        },
        {
          label: '総仕上げ(day15)',
          items: [
            { label: 'Day 15: 総仕上げ: フルスタックデプロイと最終計測', slug: 'day15' },
          ],
        },
      ],
    }),
  ],
});
