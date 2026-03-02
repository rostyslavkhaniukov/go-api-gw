import { sleep, group } from "k6";
import { TOKENS } from "./helpers/tokens.js";
import { ENDPOINTS } from "./helpers/config.js";
import {
  listUsers, listProducts,
  requestWithInvalidToken, requestWithNoAuth, requestWrongRoute,
} from "./helpers/requests.js";
import {
  checkStatus, checkRateLimit, checkUnauthorized, checkForbidden,
  errorRate, rateLimitHits,
} from "./helpers/checks.js";

export const options = {
  scenarios: {
    // Blast low-rate-key (5 req/min) to trigger 429s
    rate_limit_blast: {
      executor: "constant-vus",
      vus: 5,
      duration: "30s",
      exec: "rateLimitBlast",
      tags: { test_type: "rate_limit", scenario: "rate_limit_blast" },
    },
    // Verify other tokens are NOT affected while low-rate-key is being hammered
    rate_limit_isolation: {
      executor: "constant-vus",
      vus: 2,
      duration: "30s",
      exec: "rateLimitIsolation",
      tags: { test_type: "rate_limit", scenario: "rate_limit_isolation" },
    },
    // Auth failures under load
    auth_failures: {
      executor: "constant-vus",
      vus: 10,
      duration: "20s",
      exec: "authFailures",
      startTime: "5s",
      tags: { test_type: "auth_failure", scenario: "auth_failures" },
    },
  },
  thresholds: {
    rate_limit_429s: ["rate>0.5"],
    "checks{scenario:auth_failures}": ["rate==1"],
    "checks{scenario:rate_limit_isolation}": ["rate>0.95"],
    "http_req_duration{scenario:rate_limit_blast}": ["p(95)<500"],
    "http_req_duration{scenario:auth_failures}": ["p(95)<500"],
    errors: ["rate<0.01"],
  },
};

// Scenario 1: blast low-rate-key to exhaust its 5 req/min limit
export function rateLimitBlast() {
  group("rate-limit-blast", () => {
    const res = listUsers(TOKENS.lowRate);
    if (!checkRateLimit(res)) {
      checkStatus(res, 200, "rate-limit: non-429 is 200");
    }
  });
  sleep(0.1);
}

// Scenario 2: other tokens should not be collateral damage
export function rateLimitIsolation() {
  group("rate-limit-isolation", () => {
    checkStatus(listUsers(TOKENS.usersOnly), 200, "users-only not rate-limited");
    checkStatus(listProducts(TOKENS.productsOnly), 200, "products-only not rate-limited");
  });
  sleep(1);
}

// Scenario 3: auth failures return correct codes at high throughput
export function authFailures() {
  const roll = Math.random();

  if (roll < 0.3) {
    group("invalid-token", () => {
      checkUnauthorized(requestWithInvalidToken(ENDPOINTS.users.list), "invalid token -> 401");
    });
  } else if (roll < 0.6) {
    group("missing-auth", () => {
      checkUnauthorized(requestWithNoAuth(ENDPOINTS.products.list), "no auth -> 401");
    });
  } else if (roll < 0.8) {
    group("wrong-route", () => {
      checkForbidden(
        requestWrongRoute(TOKENS.usersOnly, ENDPOINTS.products.list),
        "users-only on products -> 403",
      );
    });
  } else {
    group("wrong-route-reverse", () => {
      checkForbidden(
        requestWrongRoute(TOKENS.productsOnly, ENDPOINTS.users.list),
        "products-only on users -> 403",
      );
    });
  }

  sleep(0.2);
}
