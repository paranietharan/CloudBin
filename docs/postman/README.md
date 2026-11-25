# Check CloudBin with Postman

1. [Start the local stack with the seed profile](../getting-started.md).
2. In Postman, choose **Import → Files** and select [CloudBin.postman_collection.json](CloudBin.postman_collection.json).
3. Open the collection’s **Variables** tab. The URLs and seed logins default to the published Compose settings. If your stack uses different ports or logins, set local values or select an environment with the overrides below.
4. Choose **Run collection**. Run the folders in order. **Sessions** signs in and creates a fresh object key. Later folders capture token IDs, the owner ID and share tokens from responses. To run individual file requests, send both sign-in requests first.

| Variables | What to set |
|---|---|
| `baseUrl` | Gateway URL, normally `http://localhost:8080` |
| `authUrl`, `objectUrl` | Direct backend URLs for health checks |
| `node1Url`, `node2Url`, `node3Url` | Individual storage-node URLs |
| `userEmail`, `userPassword`, `adminEmail`, `adminPassword` | Local seed credentials |
| `mailpitUrl` | Local Mailpit HTTP inbox, normally `http://localhost:8025` |
| `otpCode` | Current six-digit code when verifying an account manually |

## Choose how to receive verification codes

**Default Compose setup.** Run all folders through **Storage nodes** in the runner. For **Registration and password reset**, send the requests individually. After each **Begin** request, read the code for the generated `signupEmail` from `docker compose logs --tail 20 auth-service`. Skip **Find email** and **Read code**, enter the code as the local `otpCode` collection variable, then send **Verify**. Repeat with the newly issued code for the reset.

**Automated Mailpit run.** When the auth service successfully delivers OTP email to local Mailpit, run the entire collection in order. The final folder matches each email by recipient and subject, reads the OTP and submits it. Use the Mailpit HTTP inbox URL for `mailpitUrl`. SMTP credentials, STARTTLS and a trusted certificate enable local delivery. The [auth service guide](../services/auth-service.md) lists the mail settings.

## What the checks exercise

- **Health and sessions** check six HTTP services, both seed logins, owner access and anonymous access to a protected endpoint.
- **API tokens** create, list and revoke named tokens using canonical, short and legacy routes.
- **Objects and sharing** upload and overwrite a file, compare the exact bytes and SHA-256 ETag, check existence and filtering, change permissions, hide objects, use and expire share links, and delete objects.
- **Admin and legacy routes** check role enforcement, admin hide and delete, and the older file-operation aliases.
- **Storage nodes** write, read and remove a scratch file on each node.
- **Registration and password reset** exercise all three verification route variants with fresh accounts. Each flow checks that the old password stops working and the new password works.

Use a local test stack. A successful run deletes its test objects and revokes its named API tokens. Running all three registration workflows creates three accounts that remain in the test database.
