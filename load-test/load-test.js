import http from 'k6/http';
import { check, sleep } from 'k6';

// ─── Configuration ────────────────────────────────────────────────────────────
const TARGET_URL = __ENV.TARGET_URL || 'http://192.168.1.202:3010';
const DURATION  = __ENV.DURATION  || '60s';
const VUS       = parseInt(__ENV.VUS || '500');

export const options = {
  duration: DURATION,
  vus: VUS,

  // Thresholds — feel free to tune for your SLO
  thresholds: {
    http_req_duration: ['p(95)<500'],    // 95% of requests under 500ms
    http_req_failed:   ['rate<0.01'],     // error rate < 1%
    http_reqs:         ['count>1000'],   // at least 1000 total requests
  },
};

// ─── Test Parameters ──────────────────────────────────────────────────────────
const METHODS = ['GET', 'POST', 'PUT', 'DELETE'];
const PATHS   = ['/', '/health', '/api', '/api/v1', '/api/v1/users'];

// ─── Main ──────────────────────────────────────────────────────────────────────
export default function () {
  const method = METHODS[Math.floor(Math.random() * METHODS.length)];
  const path   = PATHS[Math.floor(Math.random() * PATHS.length)];
  const url    = `${TARGET_URL}${path}`;

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'User-Agent':  `k6/${__VU} Load Test`,
      'X-Request-ID': `${__VU}-${__ITER}-${Date.now()}`,
    },
  };

  let res;
  switch (method) {
    case 'GET':
      res = http.get(url, params);
      break;
    case 'POST':
      res = http.post(url, JSON.stringify({ loadtest: true, vu: __VU, iter: __ITER }), params);
      break;
    case 'PUT':
      res = http.put(url, JSON.stringify({ loadtest: true, vu: __VU, iter: __ITER }), params);
      break;
    case 'DELETE':
      res = http.del(url, null, params);
      break;
  }

  // ── Assertions ──────────────────────────────────────────────────────────────
  check(res, {
    [`[${method}] ${url} — status is 2xx/3xx`]: (r) => r.status >= 200 && r.status < 400,
    [`[${method}] ${url} — response time < 2s`]:  (r) => r.timings.duration < 2000,
  });

  // Small think time so connections aren't 100% saturated all at once
  sleep(Math.random() * 0.5); // 0–500 ms random delay
}
