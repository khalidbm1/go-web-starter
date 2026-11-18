# Go Web Starter

Ask the plugin to build a website with a Go API, a React and TypeScript frontend, Tailwind CSS, and PostgreSQL. It includes a public landing page, accounts, and a private CRUD workspace.

Example prompt:

> Build Studio Desk, a private client tracker. Include a public landing page, email sign-up, and a dashboard for creating, viewing, editing, and deleting clients.

To install the public Codex version:

```sh
codex plugin marketplace add khalidbm1/go-web-starter
codex plugin add go-web-starter@go-web-tools
```

Open a new conversation and select **Go Web Starter**. To generate a project directly from this extracted plugin directory:

```sh
python3 skills/build-website/scripts/scaffold.py ./studio-desk --name "Studio Desk" --resource client --plural clients
cd studio-desk
docker compose up --build
```

Open **http://localhost:8080**. The generated README covers API routes, tests, hosted deployment, and database persistence. For installation help or defect reports, use the Issues tab at https://github.com/khalidbm1/go-web-starter.
