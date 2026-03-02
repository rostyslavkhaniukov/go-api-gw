import { sleep } from "k6";
import { TOKENS } from "./helpers/tokens.js";
import { ENDPOINTS } from "./helpers/config.js";
import {
  listUsers, createUser, getUser, deleteUser, getUserOrders,
  listProducts, createProduct, getProduct, getProductCategories, getProductReviews,
  requestWithInvalidToken, requestWithNoAuth, requestWrongRoute,
} from "./helpers/requests.js";
import { checkStatus, checkUnauthorized, checkForbidden } from "./helpers/checks.js";

export const options = {
  scenarios: {
    smoke: {
      executor: "shared-iterations",
      vus: 1,
      iterations: 1,
      maxDuration: "30s",
    },
  },
  thresholds: {
    http_req_duration: ["p(95)<500", "p(99)<1000"],
    errors: ["rate==0"],
    checks: ["rate==1"],
  },
};

export default function () {
  // --- full-access-key: all user + product endpoints ---
  checkStatus(listUsers(TOKENS.fullAccess), 200, "full-access: list users");
  checkStatus(getUser(TOKENS.fullAccess, 1), 200, "full-access: get user");
  checkStatus(getUserOrders(TOKENS.fullAccess, 1), 200, "full-access: get user orders");
  checkStatus(createUser(TOKENS.fullAccess), 201, "full-access: create user");
  checkStatus(deleteUser(TOKENS.fullAccess, 99), 200, "full-access: delete user");
  checkStatus(listProducts(TOKENS.fullAccess), 200, "full-access: list products");
  checkStatus(getProduct(TOKENS.fullAccess, 1), 200, "full-access: get product");
  checkStatus(getProductCategories(TOKENS.fullAccess), 200, "full-access: get categories");
  checkStatus(getProductReviews(TOKENS.fullAccess, 1), 200, "full-access: get reviews");
  checkStatus(createProduct(TOKENS.fullAccess), 201, "full-access: create product");
  sleep(0.5);

  // --- users-only-key: user endpoints pass, products forbidden ---
  checkStatus(listUsers(TOKENS.usersOnly), 200, "users-only: list users");
  checkStatus(getUser(TOKENS.usersOnly, 1), 200, "users-only: get user");
  checkStatus(getUserOrders(TOKENS.usersOnly, 1), 200, "users-only: get user orders");
  checkForbidden(listProducts(TOKENS.usersOnly), "users-only: products -> 403");
  sleep(0.5);

  // --- products-only-key: product endpoints pass, users forbidden ---
  checkStatus(listProducts(TOKENS.productsOnly), 200, "products-only: list products");
  checkStatus(getProduct(TOKENS.productsOnly, 1), 200, "products-only: get product");
  checkForbidden(listUsers(TOKENS.productsOnly), "products-only: users -> 403");
  sleep(0.5);

  // --- Auth failure checks ---
  checkUnauthorized(requestWithInvalidToken(), "invalid token -> 401");
  checkUnauthorized(requestWithNoAuth(), "no auth -> 401");
  checkForbidden(
    requestWrongRoute(TOKENS.usersOnly, ENDPOINTS.products.list),
    "wrong route -> 403",
  );
}
