# Agent Notes

Work from the repo checkout root. A common local checkout path is:

```sh
~/boatkit/tugboat
```

## Local tool setup

- The Codex app shells run `zsh`.
- This repo uses `mise.toml` with Go 1.24, Node LTS, `golangci-lint latest`, and `gotestsum latest`; `mise` should activate those through the user shell startup files.

## Commands that may need elevated execution

The sandbox can block Go commands because they write outside the workspace, especially to:

- `~/Library/Caches/go-build`
- the Go module/cache directories

When a Go command fails with cache, module download, or package loading errors under the sandbox, rerun it with elevated execution approval. Useful prefixes to request are:

- `["go", "test"]`
- `["mise", "run", "test"]`
- `["mise", "run", "lint"]`
- `["mise", "run", "generate"]`
- `["golangci-lint", "run"]`
- `["gotestsum"]`
- `["go", "run"]`

Use escalation for the actual Go command rather than redirecting Go caches into the repo unless there is a specific reason to do so.

## Validation commands

CI runs:

```sh
mise run test
mise run lint
```

For focused service runner work, `gotestsum -- ./pkg/service` is a useful first check before running the full suite.

## Service runner shutdown behavior

`pkg/service.Runner` starts every registered `Activity` concurrently. On context cancellation, activity return, or another activity error, it calls `Shutdown(ctx)` and then `Kill()` for each activity.

The runner should consider an activity done only after both `Kill()` and the original `Run()` call have returned. Either can return first: `Kill()` often triggers the work inside `Run()` to stop, but `Run()` may also have returned long before `Kill()` is called. Avoid reintroducing unconditional waits on kill timeouts, because downstream apps such as Goatkit use a 5 second kill timeout and Ctrl-C should not wait that long when both `Kill()` and `Run()` stop cleanly.
