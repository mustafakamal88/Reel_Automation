# TrendCortex Frontend Deployment

## API base URL

The React/Vite frontend uses `VITE_API_BASE_URL` to decide where backend calls go.

For local development with a local Go backend, leave it empty:

```env
VITE_API_BASE_URL=
```

With an empty value, Vite proxies relative `/health`, `/platforms`, `/oauth`, `/batches`, and `/jobs` requests to `http://localhost:8080`.

For a frontend build that should call the live Railway backend, set:

```env
VITE_API_BASE_URL=https://trendcortex-api-production.up.railway.app
```

The current Railway backend URL is:

```text
https://trendcortex-api-production.up.railway.app
```

## No frontend secrets

Only public, browser-safe values may use the `VITE_` prefix. Do not put platform client secrets, API keys, session secrets, token encryption keys, OAuth access tokens, or OAuth refresh tokens in frontend environment variables, browser storage, React state, or bundled code.

OAuth and publishing credentials stay server-side in Railway environment variables and backend storage only.

## CORS

The backend currently allows the configured `APP_BASE_URL`, configured `API_BASE_URL`, and local Vite development origins (`http://localhost:5173`, `http://127.0.0.1:5173`). This keeps local frontend testing against Railway possible while the production frontend origin is not final.

When the production frontend is deployed, set `APP_BASE_URL` to that exact frontend origin and tighten CORS to the final production origin list.
