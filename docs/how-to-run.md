## Steps to run the project

## Prerequisites
- Docker
- Go version 1.26 or higher
- PostgreSQL version 15 or higher
- Redis version 7 or higher

## Running the project
1. Clone the repository:
```bash
git clone https://github.com/paranietharan/CloudBin.git
```

2. Navigate to the project directory:
```bash
cd CloudBin
```

3. Create a `.env` file in the root directory and add the necessary environment variables. You can refer to the `.env.example` file for the required variables.

4. Prepare service env files if missing:
```bash
cp api-gateway/.env.example api-gateway/.env
cp auth-service/.env.example auth-service/.env
cp object-api/.env.example object-api/.env
```

5. For storage nodes, run the deployment script:
```bash
cd scripts
```
```bash
bash deploy-storage-nodes.sh
```

![Create storage nodes](./images/create-storage-nodes.png)

6. Start all backend services from repository root:
```bash
./scripts/run-services.sh
```

7. Check service health:
```bash
./scripts/health-check.sh
```

8. Tail logs if needed:
```bash
tail -f .logs/api-gateway.log
tail -f .logs/auth-service.log
tail -f .logs/object-api.log
```

9. Stop all backend services:
```bash
./scripts/stop-services.sh
```

Manual startup (optional):
```bash
cd api-gateway
```
```bash
go run ./cmd
```

## API quick examples

Login and set JWT token:

```bash
TOKEN=$(curl -X POST "http://localhost:8080/api/v1/login" \
	-H "Content-Type: application/json" \
	-d '{"email":"<email>","password":"<password>"}' | jq -r '.token')
```

Upload file (auto-generates object key if missing):

```bash
curl -X POST "http://localhost:8080/api/v1/upload-file" \
	-H "Authorization: Bearer $TOKEN" \
	-H "Content-Type: image/png" \
	--data-binary "@/absolute/path/to/file.png"
```

List current user files:

```bash
curl "http://localhost:8080/api/v1/get-user-files" \
	-H "Authorization: Bearer $TOKEN"
```

Check if a file exists for current user:

```bash
curl "http://localhost:8080/api/v1/file-exists?object_key=<object-key>" \
	-H "Authorization: Bearer $TOKEN"
```

Make file public/private:

```bash
curl -X PUT "http://localhost:8080/api/v1/make-public-read" \
	-H "Authorization: Bearer $TOKEN" \
	-H "Content-Type: application/json" \
	-d '{"object_key":"<object-key>"}'

curl -X PUT "http://localhost:8080/api/v1/make-private-read" \
	-H "Authorization: Bearer $TOKEN" \
	-H "Content-Type: application/json" \
	-d '{"object_key":"<object-key>"}'
```

Public download for public-read file:

```bash
curl "http://localhost:8080/api/v1/public/download-file?owner_id=<owner-id>&object_key=<object-key>" -o output.bin
```

Create temporary share link (works for owner, with expiry):

```bash
curl -X POST "http://localhost:8080/api/v1/share-link" \
	-H "Authorization: Bearer $TOKEN" \
	-H "Content-Type: application/json" \
	-d '{"object_key":"<object-key>","expires_in_seconds":900}'
```

Download via temporary share link (no auth):

```bash
curl "http://localhost:8080/api/v1/share/download?token=<share-token>" -o shared-output.bin
```

Run the main integration flow script:

```bash
EMAIL="<email>" PASSWORD="<password>" FILE_PATH="/absolute/path/to/file.bin" ./scripts/integration-main-flow.sh
```

