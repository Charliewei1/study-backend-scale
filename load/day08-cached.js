// Day 8 cached load test.
//
// day07 と同条件で before/after 比較するため、負荷条件とリクエスト比率は
// load/day07-baseline.js と同じにしている。
//
// 実行方法:
//   k6 run load/day08-cached.js
//   BASE_URL=http://localhost:8080 k6 run load/day08-cached.js
//
// 見るべき指標:
//   - http_req_duration p(95): Redis キャッシュ導入前後で比較する。
//   - http_req_failed: HTTP 失敗率。1% 未満を期待する。
//   - GET /api/cache/stats: ヒット/ミス数を見てキャッシュが効いているか確認する。

import http from 'k6/http';
import { check, fail, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  scenarios: {
    baseline: {
      executor: 'ramping-vus',
      stages: [
        { duration: '30s', target: 20 },
        { duration: '1m', target: 50 },
        { duration: '30s', target: 0 },
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<100'],
    http_req_failed: ['rate<0.01'],
  },
};

function createLink(url) {
  return http.post(
    `${BASE_URL}/api/links`,
    JSON.stringify({ url }),
    {
      headers: { 'Content-Type': 'application/json' },
      tags: { name: 'POST /api/links' },
    },
  );
}

function responseCode(res) {
  try {
    return res.json('code');
  } catch (e) {
    return '';
  }
}

export function setup() {
  const links = [];

  for (let i = 0; i < 100; i += 1) {
    const url = `https://example.com/day08/seed/${i}`;
    const res = createLink(url);
    const code = responseCode(res);
    const ok = check(res, {
      'setup create returns 201': (r) => r.status === 201,
      'setup create has code': () => Boolean(code),
    });

    if (!ok) {
      fail(`failed to create setup link ${i}: status=${res.status} body=${res.body}`);
    }

    links.push({
      code,
      url,
    });
  }

  return { links };
}

export default function (data) {
  if (Math.random() < 0.9) {
    const link = data.links[Math.floor(Math.random() * data.links.length)];
    const res = http.get(`${BASE_URL}/${link.code}`, {
      redirects: 0,
      tags: { name: 'GET /{code}' },
    });

    check(res, {
      'redirect returns 302': (r) => r.status === 302,
      'redirect location matches': (r) => r.headers.Location === link.url,
    });
  } else {
    const url = `https://example.com/day08/new/${__VU}/${__ITER}`;
    const res = createLink(url);
    const code = responseCode(res);

    check(res, {
      'create returns 201': (r) => r.status === 201,
      'create has code': () => Boolean(code),
    });
  }

  sleep(1);
}
