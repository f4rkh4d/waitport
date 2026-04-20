# waitport

tiny go cli that blocks until tcp ports become reachable. useful for ci, docker healthchecks, and scripts that need to wait for stuff before booting.

basically `wait-for-it.sh` but a real binary with no bash weirdness.

## install

```
go install github.com/f4rkh4d/waitport@latest
```

or grab a binary from the releases page.

## usage

```
waitport localhost:5432 redis:6379 --timeout 30s
```

example output:

```
waiting for 2 endpoint(s) (timeout 30s)...
  ✓ localhost:5432 ready in 1.2s
  ✓ redis:6379 ready in 2.1s
all ready.
```

## flags

- `--timeout`. how long to wait total before giving up. default `60s`.
- `--interval`. how often to retry each target. default `250ms`.
- `--quiet`. shut up, only print errors.
- `-h` / `--help`. show usage.

## examples

wait for postgres then run migrations:

```
waitport db:5432 --timeout 30s && ./migrate
```

in a docker compose healthcheck-ish setup:

```
command: sh -c "waitport redis:6379 db:5432 && node server.js"
```

in ci, block until a test container is ready:

```
- run: waitport localhost:9200 --timeout 60s
```

## exit codes

- `0` all targets became reachable
- `1` timed out, or you gave it a bad argument

## why

blocks until your redis is up so your app doesnt crash at boot. drop it in your compose file and go. exits 1 if it cant reach ur port in time so ci fails loud.

stdlib only, single binary, works on linux/mac/windows.

## license

mit, see LICENSE.
