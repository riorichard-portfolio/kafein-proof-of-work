import http from 'k6/http';
import { check } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const rejectionRate = new Rate('queue_rejection_rate');
const applicationErrorRate = new Rate('application_error_rate');
const backpressureLatency = new Trend('backpressure_latency');

export const options = {
  // Aggressive ramp-up to instantly flood a small queue (QueueSize: 100)
  stages: [
    { duration: '5s', target: 100 },  // Quick jump to 100 VUs
    { duration: '20s', target: 300 }, // Massive spike exceeding queue capacity
    { duration: '20s', target: 500 }, // Massive spike exceeding queue capacity
    // { duration: '20s', target: 1000 }, // Massive spike exceeding queue capacity
    { duration: '5s', target: 0 },    // Cool-down
  ],
  thresholds: {
    // Under strict backpressure, some requests are expected to be shed/rejected (HTTP 503 or 500)
    // So we inspect the status distribution rather than demanding 0% failure.
    'http_req_duration': ['p(95)<200'],
  },
};

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
      'X-Execution-Mode': 'syncq', 
    },
  };

  let res = http.post(url, payload, params);

  // When queue is saturated, backpressure triggers error/rejection (e.g., status 503 or 500)
  let isAccepted = (r) => r.status === 200;
  let isShedded = (r) => r.status === 429;

  if (isShedded(res)) {
    // Count load shedding / backpressure rejections as expected defensive behavior
    rejectionRate.add(1);
    applicationErrorRate.add(0);
  } else if (isAccepted(res)) {
    rejectionRate.add(0);
    applicationErrorRate.add(0);
    backpressureLatency.add(res.timings.duration);
  } else {
    rejectionRate.add(0);
    applicationErrorRate.add(1);
  }
}