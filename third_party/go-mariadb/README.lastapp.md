# Lastapp driver snapshot

Source: sibling `projects/go-mariadb`, remote `git@github.com:aantich/go-mariadb.git`.

This is the dedicated MariaDB driver, module `github.com/aantich/go-mariadb`, package `mariadb`. Lastapp imports it through the root module's local replace. The snapshot includes production Go source and `internal/compression`, its module dependencies, authors and original MPL license. `SNAPSHOT.json` records SHA-256 hashes of the imported files. Lastapp does not modify those driver source files.

A snapshot is used because the sibling driver checkout is under active development. Adopt its future changes deliberately and update the recorded hashes; replace this with a reviewed release version when one is published.
