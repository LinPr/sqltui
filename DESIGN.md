# sqltui — Architecture & Feature Specification

sqltui is a terminal UI for viewing and querying tabular data. It has two modes:

- **File mode** (`sqltui [files...]`): open data files (csv/tsv/dsv/json/jsonl/parquet/excel/sqlite/fwf/logfmt/markdown/html), browse them as tabs, run SQL against them via an embedded in-memory SQLite engine.
- **Database mode** (`sqltui mysql|postgres|sqlite|redis`): connect to a live database, browse tables, run statements. This preserves the project's original functionality.

Both modes share one UI: tabbed table viewer + row-detail sheet + command palette + overlays.

## Naming discipline

Do not reference any external project by name anywhere in code, comments, docs
or identifiers. Use neutral vocabulary: frame, tab, pane, sheet, palette,
overlay, prompt.

## Directory layout

```
cmd/                    cobra CLI: root (file mode) + mysql/postgres/sqlite/redis
internal/
  data/                 Frame model (frame.go DONE) + infer.go, search.go, stats.go
  query/                embedded SQL engine over frames + SQL autocompletion
  reader/               format readers (reader.go, source.go DONE = contracts)
  writer/               format writers (writer.go DONE = contract)
  theme/                theme.go (DONE = contract) + builtin.go palettes
  config/               config.go (existing JSON config, extend with ui section)
  db/                   backend.go (DONE = contract) + mysqlbe/ postgresbe/ sqlitebe/ redisbe/
  ui/                   bubbletea app: overlay.go (DONE = contract), app/tabs/pane/tableview/sheet/statusbar/searchbar/keymap
    popup/              modal overlays (palette, query editor, exporter, ...)
    plot/               histogram + scatter rendering
```

## Bubble Tea v2 cheatsheet (VERIFIED — training data may be stale)

- Imports: `tea "charm.land/bubbletea/v2"`, `"charm.land/lipgloss/v2"`, `"charm.land/bubbles/v2/..."` (NOT github.com/charmbracelet paths).
- `type Model interface { Init() tea.Cmd; Update(tea.Msg) (tea.Model, tea.Cmd); View() tea.View }` — View returns `tea.View`, not string.
- Build views with `v := tea.NewView(content); v.AltScreen = true; v.BackgroundColor = c; v.MouseMode = ...`.
- Key input arrives as `tea.KeyPressMsg`; `msg.String()` yields `"a"`, `"A"`, `"ctrl+r"`, `"shift+left"`, `"enter"`, `"esc"`, `"f1"`, `"pgup"`, `"pgdown"`, `"home"`, `"end"`, `"tab"`. Match on those strings in a keymap layer.
- `tea.WindowSizeMsg{Width,Height}` sent initially and on resize.
- lipgloss v2: `lipgloss.Color(hex)` is a **function** returning `color.Color`, not a type. `lipgloss.NewStyle()...Render(s)` unchanged. Use `lipgloss.JoinHorizontal/JoinVertical/Place`.
- Overlay compositing: render base view, then splice the overlay box into its center manually (helper in ui package); v2.0.8 has no public layer API.
- Check any uncertain API with `go doc charm.land/bubbletea/v2 <Symbol>` before use.

## Core contracts (already written — do not change signatures without need)

- `data.Frame` / `data.Column` / `data.DType`: columnar table; cell values are exactly `nil | string | int64 | float64 | bool | time.Time`.
- `reader.Reader.Read(*Source, Options) ([]NamedFrame, error)`; register via `reader.Register(format, r)` in `init()`.
- `writer.Writer.Write(io.Writer, *data.Frame, Options) error`; register via `writer.Register`.
- `theme.Palette` (pure data) → `theme.New` derives all lipgloss styles.
- `db.Backend` (SQL engines) and `db.KVBackend` (redis) over `data.Frame`.
- `ui.Overlay { Update(tea.Msg) (Overlay, tea.Cmd); View(w, h int, th *theme.Theme) string }`; close by returning `ui.CloseOverlay` cmd; errors via `ui.ErrorMsg`; toasts via `ui.ToastMsg`.

## UI specification

### Main screen

```
┌ tab bar / status ────────────────────────────────┐
│ table view (or sheet view)                       │
│ ...                                              │
├──────────────────────────────────────────────────┤
│ status bar: Tab i/n | Row r | k x m | breadcrumb │  + search bar when active
└──────────────────────────────────────────────────┘
```

- **Tabs**: each loaded table is a tab. Each tab (pane) holds a **stack of frames**: the base frame plus one frame per applied operation (query/filter/select/order/search/cast). A breadcrumb in the status bar shows the history, e.g. `table > filter > sort`. `q` pops one level; when the stack has one frame, `q` closes the tab; closing the last tab quits.
- **Table view**: striped rows, bold header, optional row-number gutter, optional borders. Two column modes toggled with `e`: *compact* (shrink columns so everything fits) and *expanded* (natural widths, horizontal scroll, `w`/`b` jump between columns, `_`/`$` first/last column). Selected row highlighted.
- **Sheet view** (`enter`): vertical field-per-line detail of the selected row, scrollable, `c` copies row via OSC52, `q`/`esc` returns.
- **Search bar**: `/` fuzzy, `?` exact. Live filtering as you type; `enter` commits the filtered frame onto the stack, `esc` cancels.
- **Status bar**: colored tag segments — tab index, row position, shape (`rows x cols`), breadcrumb.

### Default keymap (table view)

| Key | Action |
|---|---|
| `up`/`k`, `down`/`j` | move row |
| `left`/`h`, `right`/`l` | horizontal scroll (expanded mode) |
| `g`/`home`, `G`/`end` | first / last row |
| `ctrl+u`/`ctrl+d` | half page up/down |
| `pgup`/`ctrl+b`, `pgdown`/`ctrl+f` | page up/down |
| `w` / `b` / `_` / `$` | next/prev/first/last column |
| `enter` | open sheet view |
| `e` | toggle compact/expanded columns |
| `i` | table info overlay |
| `R` | jump to random row |
| `1`-`9` | go-to-row prompt (digit prefilled) |
| `/`, `?` | fuzzy / exact search |
| `:` | command palette |
| `t` | tab switcher |
| `H`/`shift+left`, `L`/`shift+right` | prev / next tab |
| `q` | pop frame stack / close tab |
| `Q` | quit app |
| `f1` | help overlay |

### Command palette (`:`)

Fuzzy-filterable list; typing a registered prefix + space switches to an inline
prompt. Commands (name → behavior):

`query`/`q` full SQL editor with completion • `select`/`s` inline SELECT list •
`filter`/`f` inline WHERE clause • `order`/`o`/`sort` inline ORDER BY •
`search` exact search • `fuzzysearch` fuzzy search • `schema` schema browser
(all loaded tables; enter jumps) • `info` table info • `export` export wizard •
`import` import wizard • `cast` column type cast • `register` name the current
frame for SQL • `histogram` histogram builder • `scatterplot` scatter builder •
`edit` open frame in $EDITOR as CSV, reimport on save • `theme` theme selector
with live preview • `toggleborders` • `togglerownumbers` • `reset` back to base
frame • `reloadconfig` • `help` • `quit`

### SQL semantics (file mode)

- Engine: in-memory SQLite (modernc driver, already a dependency).
- Every loaded frame is registered as a table under its tab name; the current
  frame is also registered as `_` (quoted) so `select * from _ where ...` works.
- Inline prompts expand to: `SELECT <input> FROM _` / `SELECT * FROM _ WHERE <input>` / `SELECT * FROM _ ORDER BY <input>`.
- Full-query results open as a **new tab**; inline-prompt results push onto the
  current pane's stack.
- Completion: SQL keywords + column names of the current frame + registered table names.

### Database mode

- Subcommand opens a **connection form overlay** (prefilled from config, ctrl+s saves) on top of an empty workspace.
- On connect: schema browser lists namespaces/tables; selecting a table loads `SELECT * ... LIMIT 200` into a new tab.
- `:query` runs against the live backend (not the embedded engine). Non-query statements show rows-affected as a toast.
- Redis: key browser grouped by type as the schema overlay; selecting a key shows its rendered value in a sheet; `:query` prompt executes raw redis commands, result shown as pretty JSON in a sheet. Command completion from `redisbe.CommandHelps`.

## CLI (file mode flags)

`-f/--format`, `--separator`, `--quote-char`, `--no-header`, `--ignore-errors`,
`--infer-schema no|fast|safe`, `--infer-types`, `--truncate-ragged-lines`,
`--widths`, `--separator-length`, `--no-flexible-width`, `--sqlite-key`,
`--multiparts` (already wired in cmd/root.go). `-` reads stdin. http(s) URLs
download to a temp file.

## Themes

~40 built-in palettes in `internal/theme/builtin.go` (dark & light: catppuccin
variants, dracula, nord, gruvbox, solarized, tokyo-night, monokai, one-dark,
github, ayu, rose-pine, everforest, kanagawa, zenburn, material, ...).
`theme.Builtin(name)` lookup + `theme.Names()` sorted. Default `sorbet`
(warm pastel: yellow/orange leading, sky blue/mint supporting).
Selected theme + border/row-number toggles persist in the JSON config
(`~/.config/sqltui/config.json`, new `ui` section).

## Testing & verification

Every package lands with `go vet`-clean code and unit tests where logic is
non-trivial (inference, search, engine, readers, writers). `go build ./...`
must stay green after every phase.
