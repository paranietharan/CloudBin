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

## Route Mapping

### Auth Service Routes (`/api/v1/auth/*`, `/api/v1/tokens/*`)

Canonical RESTful endpoints:
- `POST /api/v1/auth/register` (Public) - Initiate registration with OTP
- `POST /api/v1/auth/register/verify` (Public) - Verify registration OTP
- `POST /api/v1/auth/login` (Public) - Authenticate and retrieve JWT
- `POST /api/v1/auth/forgot-password` (Public) - Initiate password reset with OTP
- `POST /api/v1/auth/forgot-password/verify` (Public) - Verify reset OTP & update password
- `GET /api/v1/auth/tokens` (Protected) - List user API tokens
- `POST /api/v1/auth/tokens` (Protected) - Create new API token
- `DELETE /api/v1/auth/tokens/{id}` (Protected) - Revoke API token

### Object API Routes (`/api/v1/objects/*`, `/api/v1/shares/*`, `/api/v1/public/*`, `/api/v1/admin/objects/*`)

Canonical RESTful endpoints:
- `GET /api/v1/objects` (Protected) - Paginated object list (`limit`, `offset`, `permission`, `visibility`, `key`)
- `POST /api/v1/objects` (Protected) - Upload object stream (key auto-generated or via `X-Object-Key` / query `?key=`)
- `PUT /api/v1/objects/{key...}` (Protected) - Upload/replace object with explicit key
- `GET /api/v1/objects/{key...}` (Protected) - Download object stream
- `HEAD /api/v1/objects/{key...}` (Protected) - Check object metadata & existence
- `GET /api/v1/objects/{key...}/exists` (Protected) - JSON check for object existence
- `DELETE /api/v1/objects/{key...}` (Protected) - Delete object
- `PUT|PATCH /api/v1/objects/{key...}/permission` (Protected) - Set permission (`{"permission": "public-read"|"private-read"}`)
- `PUT|PATCH /api/v1/objects/{key...}/visibility` (Protected) - Set visibility / hide object
- `POST /api/v1/objects/{key...}/shares` (Protected) - Generate temporary share link
- `GET /api/v1/shares/{token}` (Public) - Download object via temporary share token
- `GET /api/v1/public/objects/{key...}?owner_id={id}` (Public) - Download public object
- `DELETE /api/v1/admin/objects/{key...}?owner_id={id}` (Admin) - Force delete object
- `PUT|PATCH /api/v1/admin/objects/{key...}/visibility?owner_id={id}` (Admin) - Force hide object

### Legacy Aliases (Maintained for Backward Compatibility)
- Auth: `/api/v1/register`, `/api/v1/register/verify-otp`, `/api/v1/login`, `/api/v1/forgot-password`, `/api/v1/forgot-password/verify-otp`, `/api/v1/create-token`, `/api/v1/list-tokens`, `/api/v1/delete-token`
- Object: `/objects/*`, `/admin/objects/*`, `/api/v1/upload-file`, `/api/v1/download-file`, `/api/v1/delete-file`, `/api/v1/hide-file`, `/api/v1/admin/hide-file`, `/api/v1/admin/delete-file`, `/api/v1/get-user-files`, `/api/v1/file-exists`, `/api/v1/make-private-read`, `/api/v1/make-public-read`, `/api/v1/share-link`, `/api/v1/share/download`, `/api/v1/public/download-file`
