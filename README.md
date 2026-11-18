# Go Web Starter

Create a full-stack website from a prompt in Codex. The plugin generates a Go REST API, PostgreSQL database, React and TypeScript frontend, Tailwind CSS, and a Docker Compose setup. Each site has a public landing page, email/password registration and sign-in, and a private dashboard with create, read, update, and delete operations.

## Install in Codex

```sh
codex plugin marketplace add khalidbm1/go-web-starter
codex plugin add go-web-starter@go-web-tools
```

Open a new Codex conversation, select **Go Web Starter**, and try:

> Build a website called Studio Desk for tracking clients. Give it a landing page, email sign-up, and a dashboard where each user can create, view, edit, and delete their own clients.

The plugin uses the bundled project to implement the brief in your working directory. Each generated project includes its own README, migration files, REST routes, and tests.

## Generate a website directly

From a checkout of this repository, use Python 3.9+ and Docker Compose v2:

```sh
python3 plugins/go-web-starter/skills/build-website/scripts/scaffold.py ./studio-desk --name "Studio Desk" --resource client --plural clients
cd studio-desk
docker compose up --build
```

Visit **http://localhost:8080** and create an account. Anyone who can reach a deployed website can visit its landing page and register. The server restricts dashboard data to the signed-in account. The local launch binds to loopback. The generated README covers HTTPS hosting and persistence.

## Package and support

The plugin source is in `plugins/go-web-starter/`. To import a copy in Plugin Creator, use the portable ZIP in `downloads/`. This GitHub marketplace can be installed directly, independently of the public ChatGPT directory.

Report setup problems or defects using the repository's **Issues** tab. Include the failing step and relevant version numbers. Remove passwords, database URLs with credentials, session cookies, and live customer data from reports.
