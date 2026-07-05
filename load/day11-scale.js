// Day 11 HPA scale test.
//
// 実行方法:
//   1. kubectl get hpa -n shortlink -w
//   2. kubectl port-forward svc/shortlink 8080:80 -n shortlink
//   3. BASE_URL=http://localhost:8080 k6 run load/day11-scale.js
//
// kubectl get hpa -w と並べて観察し、CPU 使用率が 70% を超えた後に
// TARGETS と REPLICAS が増える様子を見る。
//
// 負荷が強すぎる/弱すぎる場合:
//   RATE=300 BASE_URL=http://localhost:8080 k6 run load/day11-scale.js
//
// 見るべき指標:
//   - kubectl get hpa -w の TARGETS と REPLICAS
//   - http_req_duration p(95): スケール前後で遅延がどう変わるか
//   - http_req_failed: 失敗率。HPA 観察中も 5% 未満を目安にする。

import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const RATE = Number(__ENV.RATE || 600);

export const options = {
  scenarios: {
    scale: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 200,
      maxVUs: 800,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500'],
    'http_req_duration{name:POST /api/links}': ['p(95)<500'],
    http_req_failed: ['rate<0.05'],
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

export default function () {
  const url = `https://example.com/day11/${__VU}/${__ITER}/${Date.now()}`;
  const createRes = createLink(url);
  const code = responseCode(createRes);

  check(createRes, {
    'create returns 201': (r) => r.status === 201,
    'create has code': () => Boolean(code),
  });

  if (code && __ITER % 20 === 0) {
    const redirectRes = http.get(`${BASE_URL}/${code}`, {
      redirects: 0,
      tags: { name: 'redirect' },
    });

    check(redirectRes, {
      'redirect returns 302': (r) => r.status === 302,
      'redirect location matches': (r) => r.headers.Location === url,
    });
  }
}
