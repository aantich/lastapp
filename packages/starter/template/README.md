# Your Lastapp client

1. Run `npm install`, then `npm run dev`. Source and compilation stay local.
2. Your agent inspects the workspace entity inventory and published App contracts through Lastapp MCP. Reuse these entities, `src/lastapp.ts`, `src/ui.tsx` and the shared stylesheet across pages.
3. Approve `http://127.0.0.1:5173` as this workspace App's origin. Click Connect, or have your connected agent issue a PKCE development launch code and call `window.lastapp.connectDevelopment({code, verifier, nonce})` using its local browser tools. Tokens stay in memory.
4. Create reusable test fixtures through the ordinary workspace App SDK. Permissions apply to fixtures too; Tenant administration does not bypass business ACLs.
5. Run `npm run build`. Get an upload ticket through MCP or upload the `dist` folder in the console's Releases tab. For terminal upload, provide its short-lived token in `LASTAPP_UPLOAD_TOKEN` and run `node scripts/upload.mjs dist <upload_url>`.
6. Capture a release with exact micro-app versions (keyed by slug) and shared package versions from your lockfile. Unchanged micro-apps keep their previous versions. Review the captured Space configuration, activate the workspace preview, and test it.
7. Production deployment needs an explicitly approved production delegation and an unchanged production base. Poll the deployment until active or failed. Backend preparation is not an atomic database/client transaction.

`lastapp.local.json` configures the local Vite server only. Hosted builds get their App ID and API origin from Lastapp's `/_lastapp/config.json`, so the same compiled files can run in development and production. Never place passwords, launch codes, App tokens or upload tickets in source/build files.

Client navigation and business workflows are ordinary React code. Add browser-compatible npm packages as needed; Node build tooling runs locally. Node-only runtime packages require a separate browser Node runtime, which is not supplied by this starter.
