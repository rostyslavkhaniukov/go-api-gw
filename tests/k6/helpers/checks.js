import { check } from "k6";
import { Rate } from "k6/metrics";

export const errorRate = new Rate("errors");
export const rateLimitHits = new Rate("rate_limit_429s");

export function checkStatus(res, expectedStatus, name) {
  const label = name || `status is ${expectedStatus}`;
  const ok = check(res, { [label]: (r) => r.status === expectedStatus });
  errorRate.add(!ok);
  return ok;
}

export function checkSuccess(res, name) {
  const label = name || "status is 2xx";
  const ok = check(res, { [label]: (r) => r.status >= 200 && r.status < 300 });
  errorRate.add(!ok);
  return ok;
}

export function checkRateLimit(res) {
  if (res.status === 429) {
    rateLimitHits.add(true);
    check(res, {
      "429 has rate limit headers": (r) =>
        r.headers["X-Ratelimit-Limit"] !== undefined,
    });
    return true;
  }
  rateLimitHits.add(false);
  return false;
}

export function checkUnauthorized(res, name) {
  return check(res, { [name || "returns 401"]: (r) => r.status === 401 });
}

export function checkForbidden(res, name) {
  return check(res, { [name || "returns 403"]: (r) => r.status === 403 });
}
