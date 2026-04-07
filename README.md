# baker

A parallel Make task runner for monorepos. Executes Make targets across multiple subdirectories concurrently with colored output and timing information.

## Installation

```bash
go install github.com/jw-develop/baker@latest
```

## Usage

```bash
baker -target <target> -title <title> -color <color> <subdirs...>
```

### Flags

| Flag | Description |
|------|-------------|
| `-target` | Make target to run (required) |
| `-title` | Title displayed in header (required) |
| `-color` | Output color: green, yellow, magenta, blue, cyan, white |

### Example

```bash
baker -target test -title "T E S T" -color green api worker database
```

Output:
```
══════════════════════════════════════════════════════════════════════════════
   ▶  T E S T
══════════════════════════════════════════════════════════════════════════════
✓ api (2.3s)
✓ worker (1.8s)
✓ database (3.1s)
══════════════════════════════════════════════════════════════════════════════
   Total time: 3.2s
══════════════════════════════════════════════════════════════════════════════
```

## Makefile Integration

```makefile
SUBDIRS := api worker database common

test:
	@baker -target test -title "T E S T" -color green $(SUBDIRS)

lint:
	@baker -target lint -title "L I N T" -color yellow $(SUBDIRS)

build:
	@baker -target build -title "B U I L D" -color blue $(SUBDIRS)
```

## Exit Codes

- `0`: All subdirectory targets succeeded
- `1`: One or more targets failed (failed output is printed inline)

## Example Project

See the `example/` directory for a complete working monorepo setup demonstrating:

```
example/
├── Makefile                    # Root orchestration with baker
├── makefiles/
│   ├── common.mk               # Aggregates all shared makefiles
│   ├── deps.mk                 # install::, tidy::
│   ├── build.mk                # build::, clean::
│   ├── test.mk                 # test::, test-nocache::
│   └── lint.mk                 # lint::
├── hello-world/
│   ├── Makefile
│   ├── go.mod
│   └── main.go
└── sup-world/
    ├── Makefile                # Extends install:: target
    ├── go.mod
    └── main.go
```

### Shared Makefiles

Each subproject includes a single `common.mk` which aggregates modular makefiles:

```makefile
# makefiles/common.mk
include $(MAKEFILE_INCLUDE_DIR)/deps.mk
include $(MAKEFILE_INCLUDE_DIR)/build.mk
include $(MAKEFILE_INCLUDE_DIR)/test.mk
include $(MAKEFILE_INCLUDE_DIR)/lint.mk
```

Subprojects include it with a relative path:

```makefile
# hello-world/Makefile
NAME := hello-world
MAKEFILE_INCLUDE_DIR ?= ../makefiles

include $(MAKEFILE_INCLUDE_DIR)/common.mk
```

### Double-Colon Targets (::)

Shared makefiles use double-colon targets (`::`) which allow subprojects to extend them:

```makefile
# makefiles/deps.mk
install::
	@go mod download
```

```makefile
# sup-world/Makefile - extends the base target
install::
	@echo "Installing additional sup-world requirements..."
```

When you run `make install` in sup-world, both rules execute. This pattern lets subprojects add custom behavior without overwriting shared logic.

### Root Makefile

The root Makefile uses baker to orchestrate parallel execution:

```makefile
SUBDIRS := hello-world sup-world

# Tip: running plain "make" from here will emulate all CI tasks!
all: tidy lint test

test:
	@baker -target test -title "T E S T" -color green $(SUBDIRS)

lint:
	@baker -target lint -title "L I N T" -color yellow $(SUBDIRS)
```

Running `make` executes tidy → lint → test sequentially, with each step running across all subdirectories in parallel.

## License

MIT
