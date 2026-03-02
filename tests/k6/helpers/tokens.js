export const TOKENS = {
  fullAccess: "full-access-key",
  usersOnly: "users-only-key",
  productsOnly: "products-only-key",
  lowRate: "low-rate-key",
};

export function authHeaders(token) {
  return { Authorization: `Bearer ${token}` };
}

export function jsonAuthHeaders(token) {
  return {
    Authorization: `Bearer ${token}`,
    "Content-Type": "application/json",
  };
}

export function pickToken(list) {
  return list[Math.floor(Math.random() * list.length)];
}

export const USER_TOKENS = [TOKENS.fullAccess, TOKENS.usersOnly];
export const PRODUCT_TOKENS = [TOKENS.fullAccess, TOKENS.productsOnly];
