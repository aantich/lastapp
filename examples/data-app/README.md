# Isolated data test App

Build the SDK with `make client`, then serve **only** the public directory:

```sh
python3 -m http.server 8789 --bind 127.0.0.1 --directory examples/data-app/public
```

Open http://localhost:8789 (the console uses 127.0.0.1, so this is a separate host).
In the Tenant console, open an App's Settings → App data access, approve
`http://localhost:8789` and its ceiling, then save. Enter that App UUID in the
example, connect, and approve the connection in the console popup.

The example uses real bearer-authenticated APIs, no mocks, persistent tokens or
console cookies. Its visibility depends on the current user's published Space
permissions and the approved App ceiling. Do not serve the repository root.
