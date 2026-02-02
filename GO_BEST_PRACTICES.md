# Go Best Practices for ccmonitor

Reference guide for writing idiomatic Go in this project. Sourced from official Go documentation and community standards.

## Project Structure

Start simple. A single `main.go` + `go.mod` is fine for small projects. Add structure only when needed.

For our project (command + internal packages):

```
ccmonitor/
  go.mod
  go.sum
  cmd/
    ccmonitor/
      main.go          # entry point
  internal/
    session/           # session file reading, parsing, PID checks
      session.go
      session_test.go
    monitor/           # TUI display logic
      monitor.go
  hooks/
    ccmonitor-hook.sh  # already exists
```

Key rules:
- `cmd/` holds main entry points. Directory name = binary name.
- `internal/` prevents external imports. Use it for everything that isn't `main`.
- **Directories = packages.** Don't create directories just for organization — each one is a new package.
- Avoid grab-bag packages like `util`, `helpers`, `models`, `common`.
- Big files are OK in Go. Don't split files prematurely.

Sources: [Official Go module layout](https://go.dev/doc/modules/layout), [Effective Go](https://go.dev/doc/effective_go)

## Naming

- **Packages**: lowercase, single word. No underscores, no mixedCaps. (`session`, `monitor`)
- **Exported names**: `MixedCaps`. Unexported: `mixedCaps`. Never underscores.
- **No `Get` prefix** on getters: `obj.Owner()` not `obj.GetOwner()`. Setters use `Set`: `obj.SetOwner(user)`.
- **Interfaces**: single-method interfaces use method name + `-er` (`Reader`, `Writer`, `Scanner`).
- **Use package context** to avoid stutter: `session.Load()` not `session.LoadSession()`.
- **Acronyms**: keep consistent casing — `HTTP`, `ID`, `URL` (all caps when exported), `httpClient` (all lower in unexported).

## Error Handling

Go treats errors as values. Handle them explicitly.

```go
// Always check errors
f, err := os.Open(name)
if err != nil {
    return fmt.Errorf("opening config: %w", err)
}
defer f.Close()
```

Rules:
- **Return errors, don't panic.** `panic` is only for truly unrecoverable bugs.
- **Wrap errors with context** using `fmt.Errorf("doing X: %w", err)`. This preserves the chain for `errors.Is` and `errors.As`.
- **Handle errors once.** If you log it, don't also return it. If you return it, don't also log it.
- **Use sentinel errors** for expected conditions: `var ErrNotFound = errors.New("session not found")`.
- **Use `errors.Is`/`errors.As`** for inspection, not type assertions.
- **Don't ignore errors** silently. If you intentionally discard one, add a comment.

## Control Flow

```go
// Guard clause pattern — handle error and return early, no else needed
if err != nil {
    return err
}
// happy path continues unindented

// If with init statement
if err := doSomething(); err != nil {
    return err
}

// Switch is more general than C — no automatic fallthrough
switch status {
case "working":
    color = green
case "waiting":
    color = yellow
default:
    color = gray
}
```

- Prefer early returns over deep nesting.
- `for range` over collections. Use `_` to discard unwanted index/value.
- No parentheses around `if`/`for`/`switch` conditions.

## Functions

- Return `(result, error)` tuples for fallible operations.
- Use `defer` for cleanup (closing files, releasing locks). Defers run LIFO.
- **Value receivers** when the method doesn't modify the receiver. **Pointer receivers** when it does, or when the struct is large.
- Keep functions short and focused. If a function does too much, split it.

## Data Structures

```go
// Composite literals — preferred over new + field assignment
s := &Session{
    ID:     "abc123",
    Status: "working",
}

// Slices over arrays
items := make([]Session, 0, 10)

// Maps — use comma-ok to distinguish missing from zero
val, ok := m[key]
if !ok {
    // key not present
}
```

## Concurrency

> "Do not communicate by sharing memory; share memory by communicating."

- Use goroutines for concurrent work: `go doWork()`.
- Use channels for communication between goroutines.
- Use `select` to wait on multiple channels.
- Use `context.Context` for cancellation and timeouts.
- For our monitor: the main loop ticks on a timer, reads files, and re-renders. Likely no goroutines needed initially.

## Testing

Go tests live next to the code: `session.go` → `session_test.go`.

```go
func TestLoadSessions(t *testing.T) {
    tests := []struct {
        name    string
        files   []string
        want    int
        wantErr bool
    }{
        {name: "empty dir", files: nil, want: 0},
        {name: "one session", files: []string{"a.json"}, want: 1},
        {name: "corrupt json", files: []string{"bad.json"}, want: 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := LoadSessions(setupTestDir(t, tt.files))
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if len(got) != tt.want {
                t.Errorf("got %d sessions, want %d", len(got), tt.want)
            }
        })
    }
}
```

Rules:
- **Table-driven tests** are idiomatic. Use `t.Run` for subtests.
- Use `t.Errorf` (continues) over `t.Fatalf` (stops) when possible — you want to see all failures.
- Use `t.TempDir()` for temp directories in tests (auto-cleaned).
- Use `t.Helper()` in test helper functions so error locations point to the caller.
- Run with `-race` flag to catch data races: `go test -race ./...`
- Test files use `_test.go` suffix. They're excluded from production builds automatically.

## Modules and Dependencies

- `go.mod` and `go.sum` are always committed.
- Run `go mod tidy` after adding/removing imports.
- Use explicit versions. Don't rely on "latest".
- Minimize dependencies. The standard library is extensive.

## Formatting and Tooling

- **`gofmt`** (or `go fmt`) handles all formatting. No debates about style.
- Tabs for indentation (enforced by `gofmt`).
- No line length limit, but wrap reasonably.
- Run `go vet` to catch common mistakes.
- `golint` / `staticcheck` for additional checks.

## TUI with Bubble Tea

We'll use [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the monitor display. It follows The Elm Architecture (Model-Update-View):

```go
type model struct {
    sessions []Session
    width    int
    height   int
}

func (m model) Init() tea.Cmd {
    return tickCmd()  // start the refresh timer
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "q" {
            return m, tea.Quit
        }
    case tickMsg:
        m.sessions = loadSessions()
        return m, tickCmd()
    }
    return m, nil
}

func (m model) View() string {
    // render the UI as a string
    return renderSessions(m.sessions)
}
```

Key points:
- All state changes happen in `Update()`. Never mutate state elsewhere.
- `View()` is a pure function: model in, string out.
- Can't log to stdout (TUI owns it). Use file logging if needed.
- Use [Lip Gloss](https://github.com/charmbracelet/lipgloss) for styling (colors, borders, alignment).
- Use [Bubbles](https://github.com/charmbracelet/bubbles) for reusable components.

## Common Patterns for This Project

### Reading JSON files gracefully
```go
data, err := os.ReadFile(path)
if err != nil {
    return nil, fmt.Errorf("reading session file: %w", err)
}

var s Session
if err := json.Unmarshal(data, &s); err != nil {
    // Skip corrupt files, don't crash
    return nil, nil
}
```

### Time formatting
```go
// Parse the ISO 8601 timestamp from the hook
t, err := time.Parse(time.RFC3339, s.LastActivity)

// "2m ago" style display
elapsed := time.Since(t)
```

### Environment variable with default
```go
dir := os.Getenv("CCMONITOR_SESSIONS_DIR")
if dir == "" {
    home, _ := os.UserHomeDir()
    dir = filepath.Join(home, ".ccmonitor", "sessions")
}
```
