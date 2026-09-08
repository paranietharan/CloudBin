# Auth Service

Auth Service manages identity, login, and JWT token lifecycle in CloudBin.

It provides:

- User registration
- User login
- Authenticated token creation (multiple JWT tokens per user)
- Token listing and token revocation

## Responsibilities

- Handle auth endpoints behind API Gateway
- Issue JWT tokens with configured issuer and TTL
- Store auth data in `auth_db`

## Required Configuration

| Variable | Required | Description |
|---|---|---|
| AUTH_PORT | Yes | HTTP port to run Auth Service |
| AUTH_DB_DSN | Yes | PostgreSQL DSN for auth database |
| JWT_SECRET | Yes | Shared JWT signing secret |
| JWT_ISSUER | Yes | JWT issuer claim |
| JWT_ACCESS_TTL | Yes | Access token lifetime (e.g. `24h`) |
| SMTP_HOST | Yes | SMTP host for Gmail, usually `smtp.gmail.com` |
| SMTP_PORT | Yes | SMTP port, usually `587` |
| SMTP_USER | Yes | Gmail address used to authenticate |
| SMTP_PASSWORD | Yes | Gmail app password |
| SMTP_FROM_EMAIL | Yes | From address shown to recipients |
| SMTP_FROM_NAME | No | Display name shown to recipients |

Example:

```env
AUTH_PORT=8081
AUTH_DB_DSN=postgres://postgres:postgres@localhost:5432/auth_db?sslmode=disable
JWT_SECRET=change-me
JWT_ISSUER=cloudbin-auth
JWT_ACCESS_TTL=24h
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-gmail-address@gmail.com
SMTP_PASSWORD=your-gmail-app-password
SMTP_FROM_EMAIL=your-gmail-address@gmail.com
SMTP_FROM_NAME=CloudBin
```

## Run Locally

```bash
cp .env.example .env
go run ./cmd
```

## Health Check

```bash
curl http://localhost:8081/healthz
```

## Database Migration

Apply auth migrations before running the service:

```bash
cd ../migrations
cp .env.example .env
go run main.go -migrate=up -target=auth
go run main.go -seed -target=auth
```

## Endpoints

### Canonical RESTful Endpoints
- `POST /api/v1/auth/register` - Initiate registration (sends OTP)
- `POST /api/v1/auth/register/verify` - Verify OTP & create user
- `POST /api/v1/auth/login` - Authenticate with email/password (returns JWT)
- `POST /api/v1/auth/forgot-password` - Initiate password reset (sends OTP)
- `POST /api/v1/auth/forgot-password/verify` - Verify OTP & update password
- `GET /api/v1/auth/tokens` - List active tokens (requires bearer token)
- `POST /api/v1/auth/tokens` - Create new API token (requires bearer token)
- `DELETE /api/v1/auth/tokens/{id}` - Revoke token by ID (requires bearer token)

### Legacy Aliases (Supported)
- `POST /api/v1/register`
- `POST /api/v1/register/verify-otp`
- `POST /api/v1/login`
- `POST /api/v1/forgot-password`
- `POST /api/v1/forgot-password/verify-otp`
- `POST /api/v1/create-token`
- `GET /api/v1/list-tokens`
- `DELETE /api/v1/delete-token`
