# Diagnose high CPU usage in a running container

Related: [issue #132](https://github.com/bitxeno/atvloadly/issues/132).

The cause of the reported idle CPU increase is not established. A Go CPU
profile captured **while the problem is happening** is needed to identify
which functions use CPU.

## Enable a local profiler

Add the environment variable `ATVLOADLY_PPROF=1` to your Docker or
Compose configuration and recreate the container. Profiling is disabled
by default. It listens only on `127.0.0.1:6060` **inside the container**,
using a separate server from the regular web UI; do not expose port 6060.

After the high CPU usage starts, run:

```sh
docker exec atvloadly sh -c 'wget -q -O /data/atvloadly-cpu.pprof "http://127.0.0.1:6060/debug/pprof/profile?seconds=30"'
```

The command saves a 30-second sample to the persistent data directory.
On a system with Go installed, inspect it with
`go tool pprof -top atvloadly-cpu.pprof`. You can also capture the
goroutine stacks with
`docker exec atvloadly sh -c 'wget -q -O /data/atvloadly-goroutines.txt "http://127.0.0.1:6060/debug/pprof/goroutine?debug=2"'`.

Compare a profile taken while idle and healthy to one taken while the
CPU load is high. Remove the environment variable and recreate the
container to disable profiling afterward.

**Privacy:** profile and goroutine files may include internal paths,
function names, and potentially sensitive strings or arguments. Review
them before sharing anything publicly; prefer sanitized function-level
summaries. This diagnostic capability does not itself fix issue #132.
