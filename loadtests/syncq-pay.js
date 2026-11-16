import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Custom metrics for distinguished metrics
const syncqLatency = new Trend('syncq_latency');
const baselineLatency = new Trend('baseline_latency');
const errorRate = new Rate('errors');

export const options = {
  // Load Testing Scenario: Gradual ramp-up leading to a "Thundering Herd"
  stages: [
    { duration: '10s', target: 50 },   // Warm-up: gradual ramp-up to 50 virtual users
    { duration: '30s', target: 200 },  // Peak Load: hit the server with 200 constant concurrent users
    { duration: '10s', target: 0 },    // Cool-down: bring back down to 0
  ],
  thresholds: {
    // Global threshold to ensure the system does not collapse completely
    'http_req_failed': ['rate<0.05'], // The overall error rate must be below 5%
    'http_req_duration': ['p(95)<500'], // 95% of requests must be completed in under 500ms
  },
};

// Ensure that this Account ID has already been seeded in your database!
const TARGET_ACCOUNT_ID = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'; 

const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000';

export default function () {
  const url = `${BASE_URL}/api/pay`;
  
  const payload = JSON.stringify({
    account_id: TARGET_ACCOUNT_ID,
    amount: 10,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      // You can change this via the k6 command line: 
      // k6 run -e MODE=syncq syncq-pay.js or k6 run -e MODE=baseline syncq-pay.js
      'X-Execution-Mode': __ENV.MODE || 'syncq', 
    },
  };

  let res = http.post(url, payload, params);

  // Validate success response
  let success = check(res, {
    'status is 200': (r) => r.status === 200,
  });

  if (!success) {
    errorRate.add(1);
  } else {
    errorRate.add(0);
    if (__ENV.MODE === 'baseline') {
      baselineLatency.add(res.timings.duration);
    } else {
      syncqLatency.add(res.timings.duration);
    }
  }
}