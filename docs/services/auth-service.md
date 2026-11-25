# Auth Service

Auth Service manages identity, login, and JWT token lifecycle in CloudBin.

Supported operations include the following.

- User registration
- User login and password reset
- Authenticated token creation (multiple JWT tokens per user)
- Token listing and token revocation

## Responsibilities

- Handle auth endpoints behind API Gateway
- Issue JWT tokens with configured issuer and TTL
- Store auth data in `auth_db`

## Configuration

| Variable | Required | Description |
|---|---|---|
| AUTH_PORT | Yes | HTTP port to run Auth Service |
| AUTH_DB_DSN | Yes | PostgreSQL DSN for auth database |
| JWT_SECRET | Yes | Shared JWT signing secret |
| JWT_ISSUER | Yes | JWT issuer claim |
| JWT_ACCESS_TTL | Yes | Access token lifetime (e.g. `24h`) |
| REDIS_ADDR | Yes | Redis for OTPs and token revocation |
| OTP_TTL | No | Lifetime of a verification code (default 5 minutes) |
| SMTP_HOST | For email | SMTP host, for example `smtp.gmail.com` |
| SMTP_PORT | For email | SMTP port, usually `587` |
| SMTP_USER | For email | SMTP username |
| SMTP_PASSWORD | For email | SMTP password |
| SMTP_FROM_EMAIL | Yes | From address shown to recipients |
| SMTP_FROM_NAME | No | Display name shown to recipients |

Example configuration

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

## Run locally from the repository root

```bash
cd auth-service
cp .env.example .env
go run ./cmd
```

## Health Check

```bash
curl http://localhost:8081/healthz
```

## Database Migration

From the repository root, apply migrations before starting the service.

```bash
cd migrations
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
- `GET /api/v1/auth/tokens` - List tokens, including revocation status (requires bearer token)
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
