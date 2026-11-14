# Lastapp database design index

The current registry schema is defined by ordered [Goose migrations](internal/control/migrations) and reproduced in [control_plane.sql](design/control_plane.sql). It includes provisioning/inventory, global identities and scoped roles, entity definitions/publications, App sessions and management-agent OAuth/delegations/receipts. Regenerate with `python3 design/validate_schema.py --write`; verify offline with `--check`.

Read [DEVELOPMENT.md](DEVELOPMENT.md) for ownership and the implementation map, [FOUNDATION.md](FOUNDATION.md) for global roles/scopes, [ENTITY_RUNTIME.md](ENTITY_RUNTIME.md) for generated business storage/ACLs, and [MCP.md](MCP.md) for agent delegation. The migrations are authoritative; this index does not maintain a second schema.

[PROVISIONING_PLAN.md](PROVISIONING_PLAN.md) and [design/VALIDATION.md](design/VALIDATION.md) preserve the earlier phase and its dated validation. Their old table counts/custom-role proposals are not the current registry specification. Current executed checks are in [VERIFICATION.md](VERIFICATION.md).
