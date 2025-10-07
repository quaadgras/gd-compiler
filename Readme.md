# gd-compiler
This is an experimental/exploratory project to assess the feasibility of compiling
Go projects into portable C11.

## Goals
- Support the full Go standard library, including generics and reflection
  (with the exception of `runtime/*` and `syscall` package functionality).
- Each source `.go` file is compiled into a corresponding `.c` file and each package
  is compiled into a pair of private + public `.h` files.
- A simple `<go.h>` header suitable for customizing the Go 'runtime' and the
  underlying implementation of builtin types.
- Enable Go code to seamlessly interoperate with cutting-edge C projects, such as
  [Cosmopolitan Libc](https://justine.lol/cosmopolitan/index.html) for cross-OS
  binary portability and [Fil-C](https://fil-c.org/) for securing unsafe/cgo and
  protecting runtime data-structures from corruption by goroutine data-races.
- cgo without the overhead, this may eventually become a more suitable compiler for
  use in [graphics.gd](https://graphics.gd).

## Caveats
The project is in a very early stage, and only supports compiling a small subset
of Go programs that don't use the standard library. In order to avoid memory leaks,
the output should be linked with a conservative garbage collector (for example,
[The Boehm-Demers-Weiser Garbage Collector](https://www.hboehm.info/gc/)) or built
with a C11 compiler with a builtin garbage collector (see. [Fil-C](https://fil-c.org/)).
