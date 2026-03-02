import http from "k6/http";
import { group } from "k6";
import { url, ENDPOINTS } from "./config.js";
import { authHeaders, jsonAuthHeaders } from "./tokens.js";

// --- User requests ---

export function listUsers(token) {
  return http.get(url(ENDPOINTS.users.list), {
    headers: authHeaders(token),
    tags: { endpoint: "GET /users", domain: "users" },
  });
}

export function createUser(token) {
  return http.post(
    url(ENDPOINTS.users.create),
    JSON.stringify({ name: "LoadTest", email: "load@test.com" }),
    {
      headers: jsonAuthHeaders(token),
      tags: { endpoint: "POST /users", domain: "users" },
    },
  );
}

export function getUser(token, id = 1) {
  return http.get(url(ENDPOINTS.users.get(id)), {
    headers: authHeaders(token),
    tags: { endpoint: "GET /users/{id}", domain: "users" },
  });
}

export function deleteUser(token, id = 1) {
  return http.del(url(ENDPOINTS.users.delete(id)), null, {
    headers: authHeaders(token),
    tags: { endpoint: "DELETE /users/{id}", domain: "users" },
  });
}

export function getUserOrders(token, id = 1) {
  return http.get(url(ENDPOINTS.users.orders(id)), {
    headers: authHeaders(token),
    tags: { endpoint: "GET /users/{id}/orders", domain: "users" },
  });
}

// --- Product requests ---

export function listProducts(token) {
  return http.get(url(ENDPOINTS.products.list), {
    headers: authHeaders(token),
    tags: { endpoint: "GET /products", domain: "products" },
  });
}

export function createProduct(token) {
  return http.post(
    url(ENDPOINTS.products.create),
    JSON.stringify({ name: "TestProduct", price: 19.99, category: "tools" }),
    {
      headers: jsonAuthHeaders(token),
      tags: { endpoint: "POST /products", domain: "products" },
    },
  );
}

export function getProduct(token, id = 1) {
  return http.get(url(ENDPOINTS.products.get(id)), {
    headers: authHeaders(token),
    tags: { endpoint: "GET /products/{id}", domain: "products" },
  });
}

export function getProductCategories(token) {
  return http.get(url(ENDPOINTS.products.categories), {
    headers: authHeaders(token),
    tags: { endpoint: "GET /products/categories", domain: "products" },
  });
}

export function getProductReviews(token, id = 1) {
  return http.get(url(ENDPOINTS.products.reviews(id)), {
    headers: authHeaders(token),
    tags: { endpoint: "GET /products/{id}/reviews", domain: "products" },
  });
}

// --- Grouped flows ---

export function userFlow(token) {
  return group("users", () => {
    const results = [];
    results.push(listUsers(token));
    results.push(getUser(token, 1));
    results.push(getUserOrders(token, 1));
    results.push(createUser(token));
    results.push(deleteUser(token, 99));
    return results;
  });
}

export function productFlow(token) {
  return group("products", () => {
    const results = [];
    results.push(listProducts(token));
    results.push(getProduct(token, 1));
    results.push(getProductCategories(token));
    results.push(getProductReviews(token, 1));
    results.push(createProduct(token));
    return results;
  });
}

export function fullFlow(token) {
  const userResults = userFlow(token);
  const productResults = productFlow(token);
  return [...userResults, ...productResults];
}

// --- Auth failure requests ---

export function requestWithInvalidToken(endpoint = ENDPOINTS.users.list) {
  return http.get(url(endpoint), {
    headers: { Authorization: "Bearer invalid-token-12345" },
    tags: { endpoint: "invalid-auth", test_type: "auth_failure" },
  });
}

export function requestWithNoAuth(endpoint = ENDPOINTS.users.list) {
  return http.get(url(endpoint), {
    tags: { endpoint: "no-auth", test_type: "auth_failure" },
  });
}

export function requestWrongRoute(token, endpoint) {
  return http.get(url(endpoint), {
    headers: authHeaders(token),
    tags: { endpoint: "wrong-route", test_type: "auth_failure" },
  });
}
