import http from 'k6/http';
import { check, sleep } from 'k6';

// ─── Configuration ────────────────────────────────────────────────────────────
const TARGET_URL = __ENV.TARGET_URL || 'http://cont.tascn.com/api/ws-2/test/health';
const DURATION  = __ENV.DURATION  || '60s';
const VUS       = parseInt(__ENV.VUS || '500');

export const options = {
  duration: DURATION,
  vus: VUS,
  thresholds: {
    http_req_duration: ['p(95)<500'],
    http_req_failed:   ['rate<0.01'],
    http_reqs:        ['count>1000'],
  },
};

export default function () {
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'User-Agent': `k6/${__VU} Load Test`,
      'X-Request-ID': `${__VU}-${__ITER}-${Date.now()}`,
    },
  };

  const res = http.get(TARGET_URL, params);

  check(res, {
    'status 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });

  sleep(0.1); // 100ms think time
}
