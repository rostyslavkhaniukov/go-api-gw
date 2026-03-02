export const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

export const ENDPOINTS = {
  users: {
    list: "/api/v1/users",
    create: "/api/v1/users",
    get: (id) => `/api/v1/users/${id}`,
    delete: (id) => `/api/v1/users/${id}`,
    orders: (id) => `/api/v1/users/${id}/orders`,
  },
  products: {
    list: "/api/v1/products",
    create: "/api/v1/products",
    get: (id) => `/api/v1/products/${id}`,
    categories: "/api/v1/products/categories",
    reviews: (id) => `/api/v1/products/${id}/reviews`,
  },
};

export function url(path) {
  return `${BASE_URL}${path}`;
}
