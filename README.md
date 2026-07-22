# scmbulk

A command-line tool to **bulk-change security and decryption rules** in Palo Alto
SCM (Strata Cloud Manager).

Every run does the same three things, in this order: **download** the current
rules → **modify** only what you asked for → **upload** the whole rule back
(never a partial write, so nothing you didn't touch gets lost). `--dry-run` lets
you see every change before anything is actually written.

## Which mode should I use?

| | Mode A — edit a CSV | Mode B — declarative `config.yaml` |
|---|---|---|
| Best for | Each rule needs a *different*, one-off edit | The *same* change across many rules |
| How you work | Download → edit cells in Excel/LibreOffice → apply | Write "match this, change that" once |
| Example | "Rename these 5 rules and tweak their descriptions individually" | "Every rule tagged `legacy` should be denied" |

Both modes go through the same safe round-trip and support `--dry-run`.

---

## Install

Download a prebuilt binary from the
[**Releases**](https://github.com/jordiolo/scmbulk/releases/latest) page (also in
the repo's right sidebar), or use these direct links (always the latest version):

| Platform | Binary |
|----------|--------|
| macOS (Apple Silicon / M1–M4) | [scmbulk-darwin-arm64](https://github.com/jordiolo/scmbulk/releases/latest/download/scmbulk-darwin-arm64) |
| macOS (Intel) | [scmbulk-darwin-amd64](https://github.com/jordiolo/scmbulk/releases/latest/download/scmbulk-darwin-amd64) |
| Windows (x64) | [scmbulk-windows-amd64.exe](https://github.com/jordiolo/scmbulk/releases/latest/download/scmbulk-windows-amd64.exe) |

On macOS, make it runnable and clear the quarantine flag:

```bash
chmod +x scmbulk-darwin-arm64
xattr -d com.apple.quarantine scmbulk-darwin-arm64   # or allow it in System Settings › Privacy & Security
./scmbulk-darwin-arm64 --help
```

On Windows, run `scmbulk-windows-amd64.exe` from PowerShell or CMD.

Or build from source (Go 1.25+):

```bash
go build -o scmbulk .
```

---

## 1. Set up your config (once)

```bash
cp config.yaml.example config.yaml
```

Fill in the `scm` block — this file is gitignored, never commit real credentials:

```yaml
scm:
  client_id:     "bulkchange@1234567890.iam.panserviceaccount.com"
  client_secret: "your-secret"
  tsg_id:        "1234567890"      # tenant ID
  folder:        "Mobile Users"    # the folder whose rules you want to change

debugenabled:  false     # verbose API logging
dryrun:        false     # force dry-run for every run (same as --dry-run)
resultsfile:             # empty = results_<timestamp>.csv

# Safety pauses (interactive)
stopfirstone:  true      # pause after the first applied rule to verify
stopevery:     25        # pause every N applied rules (0 = never)
stoponerror:   true      # on error: ask to retry / continue / abort
```

> **Non-interactive runs:** the pauses above read from the keyboard. In scripts
> or pipelines set `stopfirstone: false`, `stopevery: 0`, `stoponerror: false`
> (or pipe answers into the command), otherwise it will wait for input. If stdin
> is closed or unreadable before any input arrives (e.g. redirected from
> `/dev/null`), a pause **declines** rather than assuming "yes" — so a forgotten
> `stoponerror: true` in an unattended run fails safe instead of silently
> continuing.

> **On error, `stoponerror: true` offers three choices:** `[r]etry` redoes the
> same operation on the same rule (useful for a transient API error, e.g. an
> HTTP 503), `[c]ontinue` (the default on Enter) skips that rule and moves on,
> `[a]bort` stops the whole run.

### Security or decryption rules?

By default `scmbulk` operates on **security** rules. To work on **decryption**
rules instead, add `--type decryption` to any command, or set
`rule_type: decryption` in `config.yaml`. Everything else in this README (Mode A,
Mode B, templates, dry-run) works exactly the same for both — only the set of
editable fields differs (see [Field reference](#field-reference)).

```bash
./scmbulk download --type decryption
./scmbulk apply    --type decryption --file edited.csv --dry-run
```

If the `--type` flag and `rule_type` in the config are both set and **disagree**,
the run errors out instead of guessing — so you can't accidentally apply a
security change to decryption rules.

---

## 2. Mode A — edit a CSV

Download the rules, edit cells in a spreadsheet, apply. Each row is matched to
its rule by the `id` column. The tool **auto-diffs** every cell against the live
rule: only the cells you actually changed are written, and only for the rows you
kept in the file.

```bash
./scmbulk download                                   # -> security_<folder>_<ts>.csv
# In Excel/LibreOffice:
#   • change the cells you want
#   • delete rows you don't want to touch (optional, keeps the run focused)
#   • keep the id column intact — it identifies the rule
./scmbulk apply --file edited.csv --dry-run          # preview: field old -> new
./scmbulk apply --file edited.csv                    # apply for real
```

> `--file` accepts CSV delimited with either `,` or `;` (auto-detected from the
> header row) — handy since Excel/Numbers exports use `;` under a
> comma-decimal locale (e.g. Catalan/Spanish).

`download` flags:

| Flag | Meaning | Default |
|------|---------|---------|
| `--position pre\|post\|both` | which rulebase to download | `both` |
| `--folder <name>` | override the folder from config | config value |
| `--out <file.csv>` | output path | `<type>_<folder>_<ts>.csv` |

### Example

Downloaded row (abridged) for a rule called `Allow-Web`:

| id | name | action | source | application | tag | disabled |
|----|------|--------|--------|-------------|-----|----------|
| ab12… | Allow-Web | allow | any | web-browsing;ssl | legacy | false |

To **block it and add a `reviewed` tag**, change two cells and keep the row:

| id | name | action | source | application | tag | disabled |
|----|------|--------|--------|-------------|-----|----------|
| ab12… | Allow-Web | **deny** | any | web-browsing;ssl | **legacy;reviewed** | false |

Then:

```bash
./scmbulk apply --file edited.csv --dry-run
# * Allow-Web (pre):
#     action: "allow" -> "deny"
#     tag: "legacy" -> "legacy;reviewed"
./scmbulk apply --file edited.csv
```

Only `action` and `tag` are sent; every other field of the rule is preserved.

---

## 3. Mode B — declarative filter + change

Instead of editing a CSV, describe in `config.yaml` **which** rules to target
(`selection`) and **what** to change (`change`). Best for a uniform change over
many rules.

```bash
./scmbulk apply --select --dry-run   # preview every change, writes nothing
./scmbulk apply --select             # apply for real
```

### 3.1 `selection` — which rules to target

You can filter by an exact **list of names** (`names_file`), by **conditions on
the rule's fields** (`match`), or both together.

```yaml
selection:
  position:   both                 # pre | post | both
  names_file: target_rules.csv     # optional: only rules whose name is listed
  match:                           # optional: conditions on the live rule
    action:     allow              # exact action match
    tag:        legacy             # the rule's tag list contains this value
    name_regex: "^TEMP-"           # the rule name matches this Go regexp
```

Every entry under `match` targets one field, and the value can take any of these
forms:

| I want to match... | Write it as | Example |
|---|---|---|
| a scalar field that **equals** a value | `field: value` | `action: allow` → only rules with `action=allow` |
| a list field that **contains** a value | `field: value` | `tag: legacy` → the rule's `tag` list includes `legacy` |
| a field that is **one of several** values | `field: [a, b]` | `source: ["10.0.0.0/8", "192.168.0.0/16"]` → source is either network |
| a list containing **all** of several values (AND) | `field: {all: [a, b]}` | `source_user: {all: [ana, juan, luis]}` → the rule has all three users |
| a list containing **any** of several values (explicit OR, same as the bare list) | `field: {any: [a, b]}` | `application: {any: [ssl, web-browsing]}` |
| the rule's **name** against a pattern | `name_regex: <regexp>` | `name_regex: "^TEMP-"` → names starting with `TEMP-` |

Rules of thumb:

- Whether `field: value` means "equals" or "contains" is decided by the **live
  rule's** value: a list field uses "contains", a scalar field uses "equals" —
  you don't have to know which is which up front.
- A field the rule doesn't have **never matches** (it doesn't error, it's just
  excluded).
- All `match` entries combine with **AND**, and so does `names_file`:
  - Only `match` → every rule that satisfies all conditions.
  - Only `names_file` → exactly the rules named in that file.
  - Both → the named rules **that also** satisfy `match` (intersection).
- This works identically for `--type security` and `--type decryption`, as long
  as the field you're matching on exists for that rule type.

`names_file` is a CSV with a `name` header (or just a single column of names,
no header needed):

```csv
name
Allow-DNS
Allow-Web
Block-P2P
```

If you'd rather point `names_file` at a CSV where the name isn't the first
column — e.g. a CSV downloaded with `scmbulk download`, which starts with
`id;position;name;...` — set `names_column` to the header to read instead:

```yaml
selection:
  names_file:   security_myfolder_20260722.csv
  names_column: name
```

`names_file` accepts CSV delimited with either `,` or `;` (auto-detected from
the header row) — handy since Excel/Numbers exports use `;` under a
comma-decimal locale (e.g. Catalan/Spanish).

### 3.2 `change` — what to do to the matched rules

Three verbs:

```yaml
change:
  set:                   # replace a field's value
    action:      deny
    log_setting: "Cortex Data Lake"
  add:                   # append to a list field (only values not already present)
    tag: ["reviewed", "2026-audit"]
  remove:                # drop values from a list field
    tag: ["legacy"]
```

- `set` works on any editable field, scalar or list.
- `add` / `remove` only work on **list** fields — using them on a scalar field
  (e.g. `add: {action: [...]}`) is reported as an error for that rule, the rest
  of the run continues.
- If `add` and `remove` (or `set`) touch the **same field**, the preview and
  the results CSV report a single net change for that field (old value before
  any of them, new value after all of them) — not one line per operation. If
  they cancel out exactly, the rule is `skipped` as "no changes".
- Each value in `add`/`remove` is one **separate list item** — write
  `add: {source_hip: ["a", "b"]}` for two values, not `["a;b"]` (that would try
  to add one item literally named `a;b`, which the API will reject).

### 3.3 Complete example

> *"On the `pre` rules named `TEMP-*` that currently allow traffic: block them,
> tag them `to-remove`, and drop the `legacy` tag."*

```yaml
selection:
  position: pre
  match:
    action: allow
    name_regex: "^TEMP-"
change:
  set:
    action: deny
  add:
    tag: ["to-remove"]
  remove:
    tag: ["legacy"]
```

```bash
./scmbulk apply --select --dry-run   # review the field-by-field preview first
./scmbulk apply --select
```

### 3.4 Example: strip users and require a HIP profile

*"On every policy whose `source_user` contains user1, user2 and user3: remove
those three users and require the `HIP-Corp` HIP profile."* (`HIP-Corp` must
already exist as a HIP object in the tenant.)

```yaml
selection:
  position: both
  match:
    source_user: {all: [user1@test.com, user2@test.com, user3@test.com]}
change:
  remove:
    source_user: [user1@test.com, user2@test.com, user3@test.com]
  add:
    source_hip: ["HIP-Corp"]
```

```bash
./scmbulk apply --select --dry-run                    # security rules
./scmbulk apply --type decryption --select --dry-run  # same, for decryption rules
```

`source_user` and `source_hip` exist on both security and decryption rules, so
the same config works for either with `--type`.

---

## 4. Templates — when the new value depends on each rule

A fixed value like `action: deny` sets the same thing on every matched rule.
**Templates** let the new value be computed *per rule*, from that rule's own
current fields — so one config can produce different results for different
rules.

Any `set` / `add` / `remove` value may contain a Go
[`text/template`](https://pkg.go.dev/text/template). It's rendered **once per
matched rule**; the rule's own fields are the data, and the rendered text
becomes the new value. A value with no `{{ }}` is used as-is (no templating).

**Always wrap a template value in single quotes** in YAML, so the `{{ }}` and
any `"` inside are treated as literal text:

```yaml
change:
  set:
    description: '{{ upper .action }} rule: {{ .name }}'
```

### The data: the rule's own fields

Inside `{{ }}` you reference the rule's fields by their column name with a
leading dot: `.name`, `.action`, `.source`, `.tag`, `.source_user`,
`.log_setting`, `.disabled`, etc. — exactly the CSV columns (see
[Field reference](#field-reference)). Scalar fields are strings/booleans;
**list fields** (`.tag`, `.source`, `.from`, …) are lists — use `has`/`join`
with them, not string operators.

> Tip: run `scmbulk download` first and look at the CSV header — those column
> names are exactly what you can reference in a template.

### Helpers

| Helper | Example | Given… | Renders |
|--------|---------|--------|---------|
| `eq` / `ne` | `{{ if eq .action "allow" }}…{{ end }}` | `action=allow` | takes the branch |
| `and` / `or` | `{{ if and (eq .action "allow") (has .tag "legacy") }}…{{ end }}` | both true | takes the branch |
| `has <list> <v>` | `{{ has .tag "critical" }}` | `tag=[web,critical]` | `true` |
| `contains <s> <sub>` | `{{ contains .name "TEMP" }}` | `name=TEMP-01` | `true` |
| `lower` / `upper` | `{{ upper .action }}` | `action=allow` | `ALLOW` |
| `replace <s> <old> <new>` | `{{ replace .name "TEMP-" "prod-" }}` | `name=TEMP-01` | `prod-01` |
| `join <list> <sep>` | `{{ join .source "," }}` | `source=[a,b]` | `a,b` |
| `split <s> <sep>` | `{{ split "a;b" ";" }}` | — | list `[a,b]` |

### Worked examples

**1. Conditional on the rule's current value** — flip `allow`→`deny`, anything
else→`drop`:

```yaml
change:
  set:
    action: '{{ if eq .action "allow" }}deny{{ else }}drop{{ end }}'
```

**2. Choose a value from a tag the rule already has:**

```yaml
change:
  set:
    log_setting: '{{ if has .tag "critical" }}Full-Logging{{ else }}Basic-Logging{{ end }}'
```

**3. Build a value from another field** (e.g. document the rule in its
description):

```yaml
change:
  set:
    description: '{{ upper .action }} from {{ join .from "," }} to {{ join .to "," }}'
    # e.g. -> "ALLOW from trust to untrust"
```

**4. Combine conditions, and leave non-matching rules untouched** — echo the
current value so nothing changes when the condition is false (a no-op is
reported as `skipped`):

```yaml
change:
  set:
    # only allow+legacy rules become deny; the rest keep their action
    action: '{{ if and (eq .action "allow") (has .tag "legacy") }}deny{{ else }}{{ .action }}{{ end }}'
```

**5. Derive a list value** — `set`/`add` on a list field can produce a
`;`-separated string (remember referenced objects must already exist in the
tenant):

```yaml
change:
  add:
    tag: ['reviewed-{{ lower .action }}']   # e.g. adds "reviewed-allow"
```

### Notes & gotchas

- Templates are **Mode B only**. In Mode A you edit literal cell values.
- The rendered result is always text, then interpreted by the target field's
  type — so a template for a list field may render `a;b;c`, and one for a
  boolean must render `true`/`false`.
- Referencing a field the rule doesn't have renders `<no value>`; guard with
  `{{ if .field }}…{{ end }}` if a field may be absent.
- A template that renders the field's current value is detected as **no
  change** (that rule is `skipped`) — handy for "only touch some of the
  matched rules".
- Preview with `--dry-run` first: you'll see the rendered `old -> new` per rule.

---

## Field reference

The tool serializes each rule to a flat set of CSV columns. Any of these
columns can be edited (Mode A) or targeted (Mode B `set`/`add`/`remove`); the
write is a full round-trip, so **fields you don't touch are preserved**.

**Security rules — editable columns:**

```
name, description, policy_type, action, from, to, source, source_hip,
destination, destination_hip, source_user, application, service, category,
tag, log_setting, log_start, log_end, disabled, negate_source,
negate_destination, profile_setting, schedule, devices
```

**Decryption rules — editable columns:**

```
name, description, action (decrypt|no-decrypt), profile, from, to, source,
destination, source_user, service, category, source_hip, destination_hip,
log_setting, log_success, log_fail, disabled, negate_source,
negate_destination, tag
```

(`id` and `position` are always present but read-only — they identify the
rule.)

### How to write each cell

| Field kind | Format | Example |
|------------|--------|---------|
| Scalar / enum (`action`, `policy_type`, `description`, `log_setting`, `profile`, `schedule`) | plain text | `deny` |
| List (`source`, `destination`, `application`, `service`, `tag`, `from`, `to`, `source_user`, `category`, `source_hip`, `destination_hip`, `devices`) | values separated by `;` | `web-browsing;ssl` |
| The literal "any" | the word `any` (not an empty cell) | `any` |
| Boolean (`disabled`, `negate_source`, `negate_destination`, `log_start`, `log_end`, `log_success`, `log_fail`) | `true` / `false` (case-insensitive — Excel's `TRUE`/`FALSE` work) | `true` |
| Security profile group (`profile_setting`) | `group:<name>` | `group:best-practice` |
| Clear a field | leave the cell **empty** | *(empty)* |

Notes:

- **List order doesn't matter:** reordering `a;b` to `b;a` is not counted as a
  change.
- **Clearing:** an empty cell removes the field on write (the key is omitted
  from the PUT, which clears it server-side).
- **Reference values must already exist** in the tenant (tags, addresses,
  services, applications, zones, log profiles, profile groups). Assigning a
  name that doesn't exist fails — see [SCM API behavior](#scm-api-behavior).

### Read-only / preserved fields

Some fields are nested objects that a flat cell can't represent, so they are
**shown read-only and preserved** (never changed by the tool):

- Decryption: `type` (`ssl_forward_proxy` / `ssl_inbound_inspection` / `ssh_proxy`).
- Security: `allow_url_category`, `allow_web_application`, `log_settings`,
  `security_settings`.

Any field not listed as an editable column above is also preserved
automatically on every write.

---

## SCM API behavior

Verified against a live tenant — worth knowing:

- **The PUT is full-replace.** The tool always sends the complete rule (only
  `id` and `folder` are stripped), which is why untouched fields survive.
  Omitting a field clears it — that's how "clear a field" works.
- **The PUT is atomic.** If any part of a rule is invalid, the whole update is
  rejected and nothing in that rule changes; the error is recorded per rule in
  the results CSV.
- **Reference fields must point to existing objects.** e.g. a `tag` value must
  already exist as a Tag object, or you get `INVALID_REFERENCE`
  (`tag 'foo' is not a valid reference`). Create it in SCM first.
- **Some fields reject empty values.** `description`, for example, cannot be an
  empty string. The tool clears such fields by omitting them, not by sending
  `""`.

Because of atomicity and validation, **always run `--dry-run` first** and
review the preview and the results CSV before applying for real.

---

## Safety features

- **`--dry-run`** (or `dryrun: true`): runs the full flow but never writes to
  SCM; prints a per-field preview and writes a results CSV with
  `status=dry-run`.
- **Pauses:** `stopfirstone`, `stopevery N`, `stoponerror` ask for confirmation
  during a real run (see [non-interactive runs](#1-set-up-your-config-once)
  for how they behave without a keyboard attached).
- **Audit trail:** every run writes `results_<timestamp>.csv` with columns
  `id, name, position, status, changed_fields, message` — one row per rule, so
  you have a record of exactly what happened (`ok` / `skipped` / `dry-run` /
  `error`).

---

## License

[MIT](LICENSE) © 2026 Jordi Oliveras
