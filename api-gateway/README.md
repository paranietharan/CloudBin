# API Gateway

API Gateway is the only public-facing service in CloudBin.

It performs:

- JWT validation
- Request routing to internal services
- Basic route-level access control

## Responsibilities

- Expose public HTTP API on `GATEWAY_PORT`
- Forward auth routes to Auth Service
- Forward object routes to Object API
- Reject unauthorized requests for protected routes

## Required Configuration

| Variable | Required | Description |
|---|---|---|
| GATEWAY_PORT | Yes | HTTP port to run API Gateway |
| AUTH_SERVICE_URL | Yes | Internal base URL of Auth Service |
| OBJECT_API_URL | Yes | Internal base URL of Object API |
| JWT_SECRET | Yes | Shared JWT signing secret |
| JWT_ISSUER | Yes | Expected JWT issuer |

Example:

```env
GATEWAY_PORT=8080
AUTH_SERVICE_URL=http://localhost:8081
OBJECT_API_URL=http://localhost:8082
JWT_SECRET=change-me
JWT_ISSUER=cloudbin-auth
```

## Run Locally

```bash
cp .env.example .env
go run ./cmd
```

## Health Check

```bash
curl http://localhost:8080/healthz
```

## Route Mapping (Current)

Auth-routed paths:

- `/api/v1/register`
- `/api/v1/login`
- `/api/v1/logout`
- `/api/v1/get-user`
- `/api/v1/update-user`
- `/api/v1/delete-user`
- `/api/v1/admin/deactivate-user`
- `/api/v1/admin/delete-user`
- `/api/v1/create-token`
- `/api/v1/list-tokens`
- `/api/v1/delete-token`
- `/api/v1/get-user-files`

Object-routed paths (when Object API is running):

- `/objects/*`
- `/admin/objects/*`
- `/api/v1/upload-file`
- `/api/v1/download-file`
- `/api/v1/delete-file`
- `/api/v1/hide-file`
- `/api/v1/admin/hide-file`
- `/api/v1/admin/delete-file`
