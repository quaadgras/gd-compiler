# Conformance

Runs the upstream Go language test suite (`$GOROOT/test`, copied into `testdata/` at the
version recorded in `testdata/UPSTREAM`) through gd, and records the result of every test
case in `status.txt`.

```sh
go test ./internal/conformance                       # fails only on regressions
go test ./internal/conformance -update               # record the results in status.txt
go test ./internal/conformance -run 'TestGoRepo/ken' -v
go test ./internal/conformance -run 'TestGoRepo/fixedbugs/bug000.go' -v -keep
```

Each test case is compiled with `gd build`, the emitted C is compiled with `$GD_CC`
(default `cc`) using `$GD_CFLAGS` (default `-std=c11 -w`) and `$GD_LDFLAGS` (default
`-lm -pthread`), and then, for `// run` tests, executed and its combined output compared
against the `.out` file. `// compile` and `// build` tests only need to compile.

All of the C (emitted code and runtime) must also be portable C11, so it is first checked
with `$GD_STRICT` (default `gcc -std=c11 -pedantic-errors -fsyntax-only`, `off` to
disable). clang, and so Fil-C, accepts many extensions that other C compilers (MSVC, console
toolchains) reject. Failures at this stage are reported as `c11:`.

## status.txt

`path<TAB>pass|fail|skip<TAB>reason`, recorded with [Fil-C](https://fil-c.org/) as the C
compiler (`GD_CC=filcc GD_LDFLAGS=-static`), as the output of other C compilers can differ
slightly. A test fails `go test` only when `status.txt` says it
passes and it no longer does. Newly passing tests are listed at the end of the run.

Failure reasons are the stage (`gd`, `c11`, `cc`, `exit`, `output`) plus a normalized first line
of the error, so the backlog can be ranked by cause:

```sh
awk -F'\t' '$2=="fail"{print $3}' internal/conformance/status.txt | sort | uniq -c | sort -rn
```

Skip reasons are prefixed with a category:

- `gc:` depends on gc rather than the spec: compiler flags and diagnostics (escape analysis,
  asmcheck), runtime internals, finalizers, memstats, stack introspection, cgo, linkname,
  tests that drive the go toolchain. The rules are in `recipe.go`.
- `todo:` in scope, but the harness doesn't support the action yet (`errorcheck`, `rundir`,
  `compiledir`, `runoutput`, ...).
- `upstream:` not run upstream on this platform either.

## Updating testdata

```sh
git -C ~/git/go fetch google tag go1.X.Y --no-tags
rm -rf internal/conformance/testdata/* && git -C ~/git/go archive go1.X.Y test | tar -x -C internal/conformance/testdata --strip-components=1
```

then restore `LICENSE` and `UPSTREAM`, and rerun with `-update`.
