import { sleep } from "k6";
import { TOKENS, pickToken } from "./helpers/tokens.js";
import { fullFlow, userFlow, productFlow } from "./helpers/requests.js";
import { checkSuccess } from "./helpers/checks.js";

export const options = {
  scenarios: {
    stress: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "20s", target: 50 },
        { duration: "20s", target: 100 },
        { duration: "20s", target: 200 },
        { duration: "30s", target: 250 },
        { duration: "10s", target: 0 },
      ],
      tags: { test_type: "stress" },
    },
  },
  thresholds: {
    http_req_duration: ["p(95)<1000", "p(99)<2000"],
    "http_req_duration{domain:users}": ["p(95)<1000"],
    "http_req_duration{domain:products}": ["p(95)<1000"],
    errors: ["rate<0.05"],
    http_req_failed: ["rate<0.05"],
  },
};

export default function () {
  const token = pickToken([TOKENS.fullAccess, TOKENS.usersOnly, TOKENS.productsOnly]);

  if (token === TOKENS.usersOnly) {
    for (const r of userFlow(token)) checkSuccess(r);
  } else if (token === TOKENS.productsOnly) {
    for (const r of productFlow(token)) checkSuccess(r);
  } else {
    for (const r of fullFlow(token)) checkSuccess(r);
  }

  sleep(0.3);
}
