# Run your website

From the generated project directory, use Docker with Compose v2.

```sh
python3 setup.py
docker compose up --build
```

Visit **http://localhost:8080**, create an account, then create, open, edit, and delete your first record. The generator creates development settings automatically. Running setup again preserves your settings. Each new checkout gets its own database password. Sign out and sign in again to see your saved records.

The web container serves the Go REST API and the compiled React application on the same origin. PostgreSQL runs on the internal Docker network and stores data in a named volume. Use `docker compose down` to stop the services and `docker compose up -d` to start them again with the same data.

## Customize the site

Edit `site.json` for the brand, singular and plural display labels, example titles, and collection URL key. Update the HTML title in `web/index.html` after manually renaming a generated site. Update `web/src/main.tsx` for the landing page and authentication forms. The private dashboard, record list, detail view, and create/edit form live in `web/src/workspace.tsx`. Tailwind CSS utilities and the theme live in `web/src/style.css`.

The default record includes a title, notes, and a status of planned, active, or done. The server owns timestamps, record IDs, and account ownership. To add business fields, create a new numbered SQL migration in `migrations/`, then update `records.go`, the frontend types and forms, and the workflow test. The server applies pending migrations in filename order under a PostgreSQL transaction lock.

## API

Replace `{collection}` below with the value in `site.json`, such as `projects`. JSON writes use `Content-Type: application/json` and the header `X-Requested-With: GoWebStarter`. Send the session cookie on subsequent requests. Browsers must send writes from the configured site origin.

| Method | Path | Result |
| --- | --- | --- |
| GET | /api/health or /api/ready | Process health or database readiness |
| POST | /api/auth/register | Create an email/password account and a session |
| POST | /api/auth/login | Start a session |
| GET | /api/auth/me | Read the signed-in account |
| POST | /api/auth/logout | Revoke the session |
| GET | /api/{collection}?offset=0 | List up to 25 owned records as `{items,more}` |
| POST | /api/{collection} | Create an owned record |
| GET | /api/{collection}/{id} | Read one owned record |
| PUT | /api/{collection}/{id} | Replace its title, notes, and status |
| DELETE | /api/{collection}/{id} | Delete the owned record |

Example create or update body:

```json
{"title":"Website refresh","notes":"Prepare design review","status":"active"}
```

Successful creates return 201. Reads and updates return 200. Logout and delete return 204. Protected routes return 401 for a missing or expired session and 404 for another account's record. Account passwords use Argon2id. Opaque sessions expire after seven days, with only token hashes stored in the database.

## Development and tests

For native development, use Go 1.26+, Node.js 22.12+ and PostgreSQL 17. Set `DATABASE_URL` to your development database connection string. From the project root:

```sh
(cd web && npm ci && npm run build)
go test ./...
go run .
```

For frontend hot reload, run the backend with `APP_ORIGIN=http://localhost:5173 go run .`, then run `npm run dev` in `web/` and visit `http://localhost:5173`. Vite proxies API calls to the Go server on port 8080.

Against a disposable PostgreSQL database, run:

```sh
TEST_DATABASE_URL="$DATABASE_URL" go test -count=1 -v ./...
```

The database suite applies migrations, uses two temporary accounts, checks ownership, signs out, signs in again, reopens the application handler, and verifies persistence and deletion. Unit tests run without a database. The integration test explicitly skips when `TEST_DATABASE_URL` is unset.

## Hosting

Set `APP_ORIGIN` to the exact external HTTPS origin and terminate TLS at your reverse proxy. For the provided Compose configuration, forward requests to the loopback web port on the Docker host. Use a private database connection, strong per-environment credentials, and persistent storage with backups. The generated password is URL-safe. Percent-encode credentials when supplying a custom connection URI. Keep the database password and .env outside version control. Configure traffic limits at your proxy in addition to the application's per-email and peer limits. The server reads the immediate peer address and does not trust client-supplied forwarding headers.
