import { sleep } from "k6";
import { TOKENS, pickToken } from "./helpers/tokens.js";
import { fullFlow, userFlow, productFlow } from "./helpers/requests.js";
import { checkSuccess } from "./helpers/checks.js";

export const options = {
  scenarios: {
    spike: {
      executor: "ramping-vus",
      startVUs: 5,
      stages: [
        { duration: "5s", target: 5 },
        { duration: "3s", target: 150 },
        { duration: "20s", target: 150 },
        { duration: "3s", target: 5 },
        { duration: "15s", target: 5 },
        { duration: "5s", target: 0 },
      ],
      tags: { test_type: "spike" },
    },
  },
  thresholds: {
    http_req_duration: ["p(95)<1500", "p(99)<3000"],
    errors: ["rate<0.10"],
    http_req_failed: ["rate<0.10"],
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
