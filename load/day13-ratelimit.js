// Day 13 rate limit observation.
//
// 実行方法:
//   (cd app && RATE_LIMIT_RPS=10 RATE_LIMIT_BURST=20 go run ./cmd/server)
//   BASE_URL=http://localhost:8080 RATE=80 k6 run load/day13-ratelimit.js
//
// 意図的に RATE_LIMIT_RPS を超える POST を送り、429 の割合を見る。
// 429 は期待される防御動作なので expectedStatuses に含め、http_req_failed
// threshold では失敗扱いにしない。
//
// 見るべき指標:
//   - rate_limited: 429 がどれくらい返ったか
//   - http_req_duration p(95): 制限中も応答が速く返るか
//   - http_req_failed: 429 以外の 5xx/予期しない 4xx が増えていないか

import http from 'k6/http';
import { check } from 'k6';
import { Rate } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const RATE = Number(__ENV.RATE || 80);

export const rateLimited = new Rate('rate_limited');

http.setResponseCallback(http.expectedStatuses({ min: 200, max: 399 }, 429));

export const options = {
  scenarios: {
    exceed_rate_limit: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: '2m',
      preAllocatedVUs: 50,
      maxVUs: 200,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<250'],
    http_req_failed: ['rate<0.05'],
  },
};

export default function () {
  const res = http.post(
    `${BASE_URL}/api/links`,
    JSON.stringify({ url: `https://example.com/day13/${__VU}/${__ITER}/${Date.now()}` }),
    {
      headers: { 'Content-Type': 'application/json' },
      tags: { name: 'POST /api/links' },
    },
  );

  rateLimited.add(res.status === 429);

  check(res, {
    'create or rate limited': (r) => r.status === 201 || r.status === 429,
    '429 includes Retry-After': (r) => r.status !== 429 || Boolean(r.headers['Retry-After']),
    '429 includes limit header': (r) => r.status !== 429 || Boolean(r.headers['X-RateLimit-Limit']),
    '429 includes remaining header': (r) => r.status !== 429 || Boolean(r.headers['X-RateLimit-Remaining']),
  });
}
