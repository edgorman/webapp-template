# frontend

Vite/React single-page app, deployed to Cloudflare Pages (see the repository root `AGENTS.md` for
the full deployment architecture). It is a shell - nav bar, Google sign-in, theme toggle, error
boundary, runtime config - around the backend's example **notes** resource:

- **Feed** (`/`) — the 10 most recent notes the caller can read (every public note, plus the
  signed-in caller's own private ones), with "Load more" paging through `GET /notes`.
- **Note** (`/note/:id`) — one note, its body rendered as plain text. Its author sees an Edit link.
- **New / edit note** (`/note/new`, `/note/:id/edit`) — one editor for both, saving via
  `POST /notes` or `PUT /notes/{id}`. Requires sign-in; only the author sees the form for an
  existing note.
- **Account panel** (the account button in the nav bar) — sign in or out, and change the username
  the backend generated at first sign-in.

Every page degrades to an explanatory message rather than an error when its configuration is
missing, so the page is still useful before anything is deployed.

`public/_worker.js` is a Cloudflare Pages worker for crawlers and link previews: it serves
`robots.txt` and `sitemap.xml`, and puts a public note's title and description into the HTML a
crawler receives. It marks every response on a host other than production's `noindex`.

## Google Sign-In

Sign-in uses [Google Identity Services](https://developers.google.com/identity/gsi/web/guides/overview)
(GSI). There is no login endpoint and no session: `index.html` loads the GSI
client library, `src/hooks/useGoogleAuth.ts` initialises it and renders the
button, and on sign-in Google hands back a signed JWT credential. The hook
decodes that credential client-side **for display only** and reuses the raw
string as the bearer credential on every backend request, alongside an
`Authorization-Provider: google` header. The backend re-verifies it per request
against Google's public keys, so nothing the browser decodes is trusted.

There is no server-side session, so staying signed in across a reload is
handled in two layers:

1. **The credential is cached in `sessionStorage`** and restored on mount if it
   hasn't expired. This is the deterministic path — it doesn't depend on
   Google's session state at all. `sessionStorage` rather than `localStorage`
   so the credential dies with the tab, keeping the window in which an XSS bug
   could read it as small as possible while still surviving a reload. An
   expired entry is discarded rather than restored, since the backend would
   reject it and the UI would look signed in while every request 401s.
2. **`prompt()` asks Google to reissue one** when nothing valid was restored (a
   new tab, or the cached credential aged out). With `auto_select` that is
   silent where Google permits it — which needs a single active Google
   session, prior consent, and under FedCM no more than one sign-in attempt in
   the preceding 10 minutes. Where it isn't permitted, One Tap appears and the
   rendered button remains as the explicit fallback.

`prompt()` is what makes `auto_select` do anything: it's a One Tap setting, so
initialising with `auto_select` while only ever calling `renderButton()` leaves
it inert.

Signing out clears the cache and calls `disableAutoSelect()`, so an explicit
sign-out isn't undone by either layer on the next load.

### 1. Create an OAuth 2.0 client ID

Follow [Get your Google API client ID](https://developers.google.com/identity/gsi/web/guides/get-google-api-clientid):

1. Open the [credentials page](https://console.cloud.google.com/apis/credentials)
   for your root GCP project — the client ID is defined once in
   `infrastructure/root` and reused by every environment, so it belongs there
   rather than in the stag/prod projects.
2. Configure the OAuth consent screen if you haven't already.
3. **Create Credentials → OAuth client ID → Web application**.
4. Add every frontend URL that renders the button under **Authorized JavaScript
   origins**: `http://localhost:5173` for local dev, plus the staging and
   production site URLs.
5. Leave **Authorized redirect URIs** empty — GSI's ID-token flow returns the
   credential straight to the page's JS callback, with no server-side redirect.
6. Copy the **Client ID**. The client secret is not needed.

### 2. Configure it

Set `google_client_id` in `infrastructure/config/root/terraform.tfvars` and
apply `infrastructure/root`. That apply writes it into the `GOOGLE_CLIENT_ID`
GitHub Actions variable, and CI takes it from there for both environments with
no further per-environment configuration:

- the frontend build gets it as `VITE_GOOGLE_CLIENT_ID`;
- `infrastructure/env` gets it as `TF_VAR_google_client_id`, which becomes the
  backend Cloud Run service's `GOOGLE_CLIENT_ID` — the audience it verifies
  tokens against.

For local development, put `VITE_GOOGLE_CLIENT_ID` in
`services/frontend/.env.local` and set `GOOGLE_CLIENT_ID` in the backend's
environment, using the same client ID (with `http://localhost:5173` among its
authorized origins).

If the client ID is unset, the frontend shows a "not configured" message
instead of a button, and the backend answers any authenticated request with a
`500` rather than accepting it.

## Development

```sh
npm ci            # install
npm run lint      # oxlint
npm test          # vitest
npm run build     # tsc -b && vite build
```

`npm run dev` starts the Vite dev server directly. CI runs the same checks via moon
(`lint-check`'s `moon ci :lint :typecheck :build` and `test`'s `moon ci :test`, see .github/AGENTS.md's
"Building with moon").

CI runs `npm run build` once on merge to `main` (`frontend-build`), then uploads the same `dist/` to
both environments' buckets under a commit-SHA folder (`frontend-publish`) - so what's live always
matches a folder already sitting in the bucket, and a rollback can redeploy that folder's files
without a rebuild. The bundle carries nothing environment-specific, so identical bytes serve every
environment.

`SMOKE_BASE_URL=<site> npx playwright test` runs the post-deploy smoke test in `e2e/` against a live
site, signed out and read-only (CI passes the staging URL). `push-commit.yaml`'s
`smoke` job runs it after every staging deploy, and `pre-release` waits on it. Its files are named
`*.e2e.ts` so `npm test` never picks them up; first run `npx playwright install chromium` outside a
Claude Code cloud session, which has one preinstalled.

## Configuration

| Env var                     | Description                                              |
| ---------------------------- | --------------------------------------------------------- |
| `VITE_BACKEND_URL`           | Fallback base URL of the backend service. Unset in CI; only used in local dev and for a rollback to a build that predates `config.json`. |
| `VITE_GOOGLE_CLIENT_ID`      | Google OAuth 2.0 client ID. Not a secret; baked in at build time; see above. |

Everything else is **runtime** configuration: `src/lib/config.ts` fetches `/config.json` once at
bootstrap, before the app's first render, so the same published folder can serve any environment
just by which `config.json` sits next to its static files. `frontend-deploy` writes it as part of
both the staging and promotion pipelines - after the folder is fetched, and never uploaded, so the
stored folder stays environment-neutral:

| Field         | Value                                                    | Without config.json  |
| ------------- | -------------------------------------------------------- | -------------------- |
| `backendUrl`  | the just-deployed backend's URL                          | `VITE_BACKEND_URL`   |
| `siteUrl`     | production's URL (the `PRODUCTION_URL` repository variable), on every environment, so canonical links never point at staging | the page's origin |
| `siteName`    | the `SITE_NAME` repository variable                      | `My App`             |
| `version`     | the release tag this build is or becomes                 | absent               |
| `environment` | `staging` or `production`                                | absent               |
| `githubRepo`  | `owner/name` of this repository                          | absent               |

`version` and `environment` drive the footer pinned to the bottom of AccountPanel's drawer: the
version links to its GitHub Releases page, and only staging shows a badge.

`VITE_GOOGLE_CLIENT_ID` stays build-time only: one client ID serves every environment (see above),
so there is nothing for a runtime config to vary.
