import http from 'k6/http';
import { check } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics for distinguished metrics
const syncqLatency = new Trend('syncq_latency');
const baselineLatency = new Trend('baseline_latency');
const errorRate = new Rate('errors');
const amount = 10

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
const EXECUTION_MODE = __ENV.MODE || 'syncq';

// 1. SETUP: Fetch initial account balance and transaction count before load test starts
export function setup() {
    const url = `${BASE_URL}/api/acc/${TARGET_ACCOUNT_ID}`;
    let res = http.get(url);

    if (res.status !== 200) {
        throw new Error(`Failed to fetch initial account state: ${res.status} - ${res.body}`);
    }

    const initialData = res.json();
    console.log(`--- [SETUP] Initial Balance: ${initialData.actual_balance_in_db}, Initial Total Transactions: ${initialData.total_successful_payments_recorded} ---`);

    return {
        initialBalance: initialData.actual_balance_in_db,
        initialCount: initialData.total_successful_payments_recorded,
    };
}

// 2. DEFAULT: Main load testing function
export default function (data) {
    const url = `${BASE_URL}/api/pay`;

    const payload = JSON.stringify({
        account_id: TARGET_ACCOUNT_ID,
        amount, // Amount per transaction
    });

    const params = {
        headers: {
            'Content-Type': 'application/json',
            // You can change this via the k6 command line: 
            // k6 run -e MODE=syncq syncq-pay.js or k6 run -e MODE=baseline syncq-pay.js
            'X-Execution-Mode': EXECUTION_MODE,
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
        if (EXECUTION_MODE === 'baseline') {
            baselineLatency.add(res.timings.duration);
        } else {
            syncqLatency.add(res.timings.duration);
        }
    }
}

// 3. TEARDOWN: Financial reconciliation and data integrity validation after load test finishes
export function teardown(data) {
    const url = `${BASE_URL}/api/acc/${TARGET_ACCOUNT_ID}`;
    let res = http.get(url);

    if (res.status !== 200) {
        console.error(`Failed to fetch final account state: ${res.status} - ${res.body}`);
        return;
    }

    const finalData = res.json();

    const expectedCountIncrement = finalData.total_successful_payments_recorded - data.initialCount;
    const expectedBalanceIncrement = finalData.actual_balance_in_db - data.initialBalance;

    // Since each request sends amount 
    const expectedSumDelta = expectedCountIncrement * amount;

    console.log(`\n========================================`);
    console.log(`--- [RECONCILIATION AUDIT REPORT] ---`);
    console.log(`Execution Mode          : ${EXECUTION_MODE.toUpperCase()}`);
    console.log(`Total Recorded Tx Delta : ${expectedCountIncrement} transactions`);
    console.log(`Expected Amount Delta   : ${expectedSumDelta}`);
    console.log(`Actual Balance Delta    : ${expectedBalanceIncrement}`);
    console.log(`========================================\n`);

    if (expectedSumDelta === expectedBalanceIncrement) {
        console.log(`✅ [SUCCESS] Data 100% Accurate! Zero Lost Updates detected.`);
    } else {
        console.error(`❌ [MISMATCH] Financial discrepancy detected! Potential lost updates occurred.`);
    }
}