import { sleep } from "k6";
import { TOKENS, pickToken, USER_TOKENS, PRODUCT_TOKENS } from "./helpers/tokens.js";
import { userFlow, productFlow, fullFlow } from "./helpers/requests.js";
import { checkSuccess } from "./helpers/checks.js";

export const options = {
  scenarios: {
    load: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "20s", target: 50 },
        { duration: "1m", target: 50 },
        { duration: "10s", target: 0 },
      ],
      tags: { test_type: "load" },
    },
  },
  thresholds: {
    http_req_duration: ["p(95)<300", "p(99)<500"],
    "http_req_duration{domain:users}": ["p(95)<300"],
    "http_req_duration{domain:products}": ["p(95)<300"],
    errors: ["rate<0.01"],
    http_req_failed: ["rate<0.01"],
    checks: ["rate>0.99"],
  },
};

export default function () {
  const roll = Math.random();

  if (roll < 0.4) {
    // 40%: full flow with full-access token
    for (const r of fullFlow(TOKENS.fullAccess)) checkSuccess(r);
  } else if (roll < 0.65) {
    // 25%: user flow with random user-capable token
    for (const r of userFlow(pickToken(USER_TOKENS))) checkSuccess(r);
  } else if (roll < 0.9) {
    // 25%: product flow with random product-capable token
    for (const r of productFlow(pickToken(PRODUCT_TOKENS))) checkSuccess(r);
  } else {
    // 10%: mixed contention — pick any token, run its allowed flow
    const token = pickToken([TOKENS.fullAccess, TOKENS.usersOnly, TOKENS.productsOnly]);
    if (token === TOKENS.usersOnly) {
      for (const r of userFlow(token)) checkSuccess(r);
    } else if (token === TOKENS.productsOnly) {
      for (const r of productFlow(token)) checkSuccess(r);
    } else {
      for (const r of fullFlow(token)) checkSuccess(r);
    }
  }

  sleep(0.5);
}
