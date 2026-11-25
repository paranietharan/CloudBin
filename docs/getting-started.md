# Run CloudBin locally

From the repository root, start Docker Compose with the seed profile to create the admin and demo accounts used by the Postman collection.

```sh
docker compose --profile seed up --build -d
docker compose --profile seed ps -a
curl http://localhost:8080/healthz
```

The migration and seed jobs should exit with code 0. The gateway, auth service, object API, three storage nodes, two PostgreSQL servers, Redis and Mailpit stay running. To start without demo accounts, use `docker compose up --build -d`.

| Local service | Default host port |
|---|---|
| API gateway | 8080 |
| Auth service / Object API | 8081 / 8082 |
| Storage nodes 1, 2 and 3 | 8083, 8084, 8085 |
| Mailpit web inbox / SMTP | 8025 / 1025 |

The default seed logins are `admin@example.com` / `admin123` and `user@example.com` / `user123`. To customize them, set `ADMIN_EMAIL`, `ADMIN_INITIAL_PASSWORD`, `USER_EMAIL` and `USER_INITIAL_PASSWORD` before the first seed. Seeding skips existing accounts. The gateway, auth service and object API must share the same `JWT_SECRET` and `JWT_ISSUER`. [Compose defaults](../docker-compose.yml) · [Example environment](../.env.example)

## Verification codes

With the default empty SMTP credentials, the auth service writes the issued code to its log. In local development, read it with `docker compose logs --tail 20 auth-service`. When SMTP is configured, use the registered address's inbox. Local Mailpit delivery with credentials requires STARTTLS and a certificate for `mailpit` that the auth container trusts. The [Postman guide](postman/README.md) covers both ways to verify accounts.

## Build, check and stop

A single image uses the repository root as its build context so Docker can include the shared Go packages.

```sh
docker build -f object-api/Dockerfile .
for module in api-gateway auth-service migrations object-api storage-node shared; do
  (cd "$module" && go test -race ./... && go vet ./...) || exit 1
done
```

Run `docker compose --profile seed down` to stop the stack and keep its data volumes. For a fresh installation, `docker compose --profile seed down --volumes --remove-orphans` also deletes the local databases, Redis data and stored files.

[Historical overwrite and migration checks](https://github.com/paranietharan/CloudBin/actions/runs/44537147386)
