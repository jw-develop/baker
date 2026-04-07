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

## License

MIT
