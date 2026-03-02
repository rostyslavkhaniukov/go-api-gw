# Senior / Lead Go Engineer -- Home Assignment

## API Gateway Service Assignment

## Overview

Create a simple API Gateway/Proxy service that demonstrates your ability
to build production-ready microservices in Go. The service should handle
API key validation, rate limiting with cross-instance synchronization,
and request proxying.

------------------------------------------------------------------------

## Core Requirements

### 1. Proxy Service

-   Forward HTTP requests to configurable backend services\
-   Support path-based routing\
-   Handle request/response headers properly\
-   Show good usage of middlewares

### 2. API Protection

-   The proxy must protect backend APIs by validating access tokens on
    each request\
-   Access tokens should be provided as HTTP headers
    -   Example: `Authorization: Bearer <token>`\
-   Token data (including rate limits, expiry, and allowed routes)
    should be stored in Redis\
-   Support rate limiting per token\
-   Handle token expiration\
-   Be clear in your implementation how tokens are created/provided
    -   You may use static tokens, generate them with a script, or use
        certificates\
    -   Describe your approach in the README

### 3. Rate Limiting

-   Requests must be rate limited per token\
-   Distributed rate limiting (synchronization across multiple
    instances) is optional\
-   If implemented, highlight your approach

### 4. Technical Requirements

-   Written in Go 1.21\
-   Redis for token storage and rate limit synchronization\
-   Environment-based configuration\
-   Concurrent request handling\
-   Proper error handling and logging

------------------------------------------------------------------------

## Required Endpoints

### Proxy Endpoint

    POST /api/v1/*

### Token Data Structure

``` json
{
  "api_key": "xxx-xxx-xxx",
  "rate_limit": 100,
  "expires_at": "2024-12-31T23:59:59Z",
  "allowed_routes": [
    "/api/v1/users/*",
    "/api/v1/products/*"
  ]
}
```

------------------------------------------------------------------------

## Minimum Deliverables

-   Working proxy service with token validation and rate limiting\
-   Dockerfile\
-   Unit tests\
-   Basic documentation\
-   Optional: Implementation of distributed rate limiting

------------------------------------------------------------------------

## Bonus Features (Optional)

-   Health check and readiness endpoints\
-   Metrics endpoint\
-   Helm charts\
-   OpenAPI documentation\
-   Integration tests\
-   Circuit breaker\
-   Prometheus metrics integration

------------------------------------------------------------------------

## Development Approach

You are encouraged to use all professional resources, including
documentation, libraries, and modern development tools. Leveraging AI
assistants for code generation, architectural guidance, or
problem-solving is acceptable. The focus is on your problem-solving
approach and final solution quality.

------------------------------------------------------------------------

## Time Limit

Focus on demonstrating how you structure and design your solution. The
core functionality could be completed in **2--3 hours**, but
prioritizing clean, well-thought-out code is more important than
implementing every feature.

------------------------------------------------------------------------

## Submission

-   Include documentation in `README.md` or via GoDocs\
-   Ensure code is well-commented\
-   Include build and run instructions\
-   Submit preferably as a GitHub repository

------------------------------------------------------------------------

## What They're Looking For

-   Clean, well-structured code\
-   Proper handling of concurrent operations\
-   Thoughtful error handling\
-   Critical area testing coverage\
-   Clear documentation\
-   Effective distributed system design (if attempted)

------------------------------------------------------------------------

## Note on Submission

Highlight any potential inconsistencies or your interpretation of the
requirements. Demonstrating critical analysis reflects real-world
engineering scenarios.

