// Day 7 baseline load test.
//
// 実行方法:
//   k6 run load/day07-baseline.js
//   BASE_URL=http://localhost:8080 k6 run load/day07-baseline.js
//
// 見るべき指標:
//   - http_req_duration p(95): 遅い側 5% を除いた応答時間。まず 100ms 未満を目標にする。
//   - http_req_failed: HTTP 失敗率。1% 未満を期待する。
//   - iterations / http_reqs: VU 数に対してどれだけ処理できたかを見る。

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
    'http_req_duration{name:redirect}': ['p(95)<100'],
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
    const url = `https://example.com/load/seed/${i}`;
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
  if (__ITER % 10 === 0) {
    const url = `https://example.com/load/new/${__VU}/${__ITER}`;
    const res = createLink(url);
    const code = responseCode(res);

    check(res, {
      'create returns 201': (r) => r.status === 201,
      'create has code': () => Boolean(code),
    });
  } else {
    const link = data.links[__ITER % data.links.length];
    const res = http.get(`${BASE_URL}/${link.code}`, {
      redirects: 0,
      tags: { name: 'redirect' },
    });

    check(res, {
      'redirect returns 302': (r) => r.status === 302,
      'redirect location matches': (r) => r.headers.Location === link.url,
    });
  }

  sleep(1);
}
