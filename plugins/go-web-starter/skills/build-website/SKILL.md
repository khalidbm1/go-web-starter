---
name: build-website
description: Build or extend a full-stack website with a Go REST API, PostgreSQL, React, TypeScript, Tailwind CSS, a public landing page, accounts, and private CRUD workflows. Use the bundled starter when implementing a website from a brief.
---

# Build a website

Deliver working source in the user's project. Extract the site name, primary resource, users, brand voice, and any required fields from the brief. If a detail is missing and easy to change, use a sensible default. With no domain specified, use Fieldbook and projects.

## Generate the first working feature

Resolve the paths below relative to this SKILL.md. For a new project, run the bundled generator with Python 3.9 or newer, supplying the chosen destination. The destination may be new or empty.

```sh
python3 scripts/scaffold.py ./fieldbook --name "Fieldbook" --resource project --plural projects
```

The script copies the bundled project and writes a fresh development database password into its ignored .env. Use a descriptive name and singular/plural URL keys. Optional --label and --plural-label control the displayed names independently. The generated site.json supplies the title, labels, examples, and API collection key. The default record contains a title, notes, and status. Update the landing copy, sample entries, and workflow to reflect the brief.

For an existing nonempty repository, first inspect its stack, conventions, and existing authentication. Generate into a new temporary directory and selectively integrate the missing pieces. Preserve unrelated work, live data, and established migration history. Never replace an existing project with the starter. A host that exposes package files as resources can materialize the template tree and script together into a writable scratch directory first.

## Implement the requested domain

Run through the actual user journey. A visitor can read the landing page and register. A signed-in user can create a record, open its detail view, edit it, delete it after confirmation, and sign out. Ownership comes from the server-side session. Every single-record lookup and mutation must enforce ownership.

For additional fields, change the database migration, Go request and response types and validation, SQL, React form and detail view, and tests together. For an already-created database, add a new numbered SQL migration. The migrator records applied file names. Preserve the version of any migration already applied. Use independent, explicit data relationships for new resource types. Match responsive layouts, labels, error states, and landing content to the product the user described.

## Run and deliver

Follow the generated README.md for prerequisites, Docker Compose, development, the API contract, and tests. Run the frontend type check and build, Go tests, and integration tests against a disposable PostgreSQL database when available. Exercise the public page, sign-up, sign-in, all four record operations, persistence, account isolation, and sign-out. Tell the user exactly which checks ran and any environmental limitation.

Return the project location and the simplest way to start it. In a hosted coding environment, return a downloadable project when the user needs files. Generate fresh local credentials for the recipient; keep them out of shared archives and have the recipient run python3 setup.py to create their own .env. Deploy only when requested, using the user's target. For a public deployment configure the HTTPS origin, private database connectivity, persistent storage and the hosting provider's secrets.

## Profiles, administration, and additional modules

Every new site includes a private user profile, a personal dashboard scoped to each module, and an administrator dashboard with user management. Keep sign-up accounts at the user role. Bootstrap the first administrator for a requested test deployment with the generated operator command and a dedicated test account. Show the owner how to promote their own account without shipping a default administrator password.

For each additional CRUD resource in the brief, run the generated project's command, for example `python3 manage.py add-resource invoice --plural invoices`. Rebuild after adding modules. The shared router and typed UI expose separate collections with owner- and module-scoped queries. Extend the migration, request validation, and forms when the brief needs business-specific fields. Test the profile workflow, administrator and ordinary-user access, user lifecycle, and two-module isolation.
