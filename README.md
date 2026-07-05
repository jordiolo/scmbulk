# scmbulk

A command-line tool to **bulk-change security and decryption rules** in Palo Alto
SCM (Strata Cloud Manager).

It downloads the rules of a folder, lets you change them in bulk, and writes them
back — always through a safe GET → modify → PUT round-trip, with a `--dry-run`
preview so you see every change before anything is written.

There are two ways to make changes:

- **Mode A — edit a CSV:** download the rules, edit cells in Excel/LibreOffice,
  apply. Best when each rule needs different edits.
- **Mode B — declarative rules in `config.yaml`:** describe *which* rules to
  match and *what* to change (with optional templates). Best for one uniform
  change across many rules.

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

## Quick start

```bash
# 1. Create your config and fill in credentials + folder
cp config.yaml.example config.yaml
#    edit config.yaml -> scm: { client_id, client_secret, tsg_id, folder }

# 2. Download the folder's security rules to a CSV
./scmbulk download                       # writes security_<folder>_<timestamp>.csv

# 3. Open the CSV, change the cells you want, save it

# 4. Preview the changes (nothing is written to SCM)
./scmbulk apply --file security_myfolder_20260705_120000.csv --dry-run

# 5. Apply for real
./scmbulk apply --file security_myfolder_20260705_120000.csv
```

That's Mode A. For Mode B (declarative), see below.

---

## Configure

Copy `config.yaml.example` to `config.yaml` and fill in the `scm` block. The file
is gitignored — never commit real credentials.

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
stoponerror:   true      # pause and ask whether to continue when a rule errors
```

> **Non-interactive runs:** the pauses above read from the keyboard. In scripts
> or pipelines set `stopfirstone: false`, `stopevery: 0`, `stoponerror: false`
> (or pipe answers into the command), otherwise it will wait for input.

---

## Rule type: security or decryption

By default `scmbulk` operates on **security** rules. To work on **decryption**
rules, pass `--type decryption` or set `rule_type: decryption` in `config.yaml`.

```bash
./scmbulk download --type decryption
./scmbulk apply    --type decryption --file edited.csv --dry-run
```

```yaml
# config.yaml — alternative to the flag
rule_type: decryption
```

If the flag and the config are both set and **disagree**, the run errors out
(so you can't accidentally apply a security change to decryption rules).

The workflow (download / Mode A / Mode B / dry-run) is identical for both types;
only the set of editable fields differs — see [Field reference](#field-reference).

---

## Mode A — edit a CSV

Download the rules, edit the cells in a spreadsheet, and apply. Each row is
matched to its rule by the `id` column, and the tool **auto-diffs** every cell
against the live rule: only the cells you actually changed are written, and only
to the rules you kept in the file.

```bash
./scmbulk download                                   # security_<folder>_<ts>.csv
# In Excel/LibreOffice:
#   • change the cells you want
#   • delete rows you don't want to touch (optional, keeps the run focused)
#   • keep the id column intact — it identifies the rule
./scmbulk apply --file edited.csv --dry-run          # preview: field old -> new
./scmbulk apply --file edited.csv                    # apply for real
```

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

## Mode B — declarative filter + change

Instead of editing a CSV, describe in `config.yaml` **which** rules to target
(`selection`) and **what** to change (`change`). Best for a uniform change over
many rules.

```bash
./scmbulk apply --select --dry-run   # preview every change, writes nothing
./scmbulk apply --select             # apply for real
```

### `selection` — which rules

```yaml
selection:
  position:   both                 # pre | post | both
  names_file: target_rules.csv     # optional: only rules whose name is listed
  match:                           # optional: conditions on the live rule
    action:     allow              # exact action match
    tag:        legacy             # the rule's tag list contains this value
    name_regex: "^TEMP-"           # the rule name matches this Go regexp
```

All given criteria combine with **AND**:

- With only `match`: every rule that satisfies all conditions.
- With only `names_file`: exactly the rules named in that file.
- With both: the named rules **that also** satisfy `match` (intersection).

`names_file` is a CSV with a `name` header (or a single column of names):

```csv
name
Allow-DNS
Allow-Web
Block-P2P
```

### `change` — what to change

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

- `set` works on any editable field.
- `add` / `remove` only work on **list** fields; using them on a scalar field is
  reported as an error for that rule.

### Complete example

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

### Templates (dynamic values)

Every `set` / `add` / `remove` value may contain a Go
[`text/template`](https://pkg.go.dev/text/template), rendered **once per rule**
with that rule's own fields as data (`.name`, `.action`, `.source`, `.tag`, …).
List fields are exposed as string lists. A value with no `{{ }}` is used literally.

Helpers:

| Helper | Example | Result |
|--------|---------|--------|
| `has <list> <v>` | `{{ has .tag "critical" }}` | `true` if the tag list contains `critical` |
| `contains <s> <sub>` | `{{ contains .name "TEMP" }}` | substring test |
| `lower` / `upper` | `{{ lower .name }}` | change case |
| `replace <s> <old> <new>` | `{{ replace .name "TEMP-" "" }}` | string replace |
| `join <list> <sep>` | `{{ join .source "," }}` | list → string |
| `split <s> <sep>` | `{{ split "a;b" ";" }}` | string → list |
| `eq`, `ne`, `and`, `or`, `if`/`else` | built-ins from `text/template` | conditionals |

Example — set the value based on each rule's current state:

```yaml
change:
  set:
    # flip allow → deny, anything else → drop
    action: '{{ if (eq .action "allow") }}deny{{ else }}drop{{ end }}'
    # pick a log profile based on a tag the rule already has
    log_setting: '{{ if (has .tag "critical") }}Cortex-Full{{ else }}Cortex-Basic{{ end }}'
```

---

## Field reference

The tool serializes each rule to a flat set of CSV columns. Any of these columns
can be edited (Mode A) or targeted (Mode B `set`/`add`/`remove`); the write is a
full round-trip, so **fields you don't touch are preserved**.

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

(`id` and `position` are always present but read-only — they identify the rule.)

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

- **List order doesn't matter:** reordering `a;b` to `b;a` is not counted as a change.
- **Clearing:** an empty cell removes the field on write (the key is omitted from
  the PUT, which clears it server-side).
- **Reference values must already exist** in the tenant (tags, addresses,
  services, applications, zones, log profiles, profile groups). Assigning a name
  that doesn't exist fails — see [SCM API behavior](#scm-api-behavior).

### Read-only / preserved fields

Some fields are nested objects that a flat cell can't represent, so they are
**shown read-only and preserved** (never changed by the tool):

- Decryption: `type` (`ssl_forward_proxy` / `ssl_inbound_inspection` / `ssh_proxy`).
- Security: `allow_url_category`, `allow_web_application`, `log_settings`,
  `security_settings`.

Any field not listed as an editable column above is also preserved automatically
on every write.

---

## SCM API behavior

Verified against a live tenant — worth knowing:

- **The PUT is full-replace.** The tool always sends the complete rule (only `id`
  and `folder` are stripped), which is why untouched fields survive. Omitting a
  field clears it — that's how "clear a field" works.
- **The PUT is atomic.** If any part of a rule is invalid, the whole update is
  rejected and nothing in that rule changes; the error is recorded per rule in
  the results CSV.
- **Reference fields must point to existing objects.** e.g. a `tag` value must
  already exist as a Tag object, or you get `INVALID_REFERENCE`
  (`tag 'foo' is not a valid reference`). Create it in SCM first.
- **Some fields reject empty values.** `description`, for example, cannot be an
  empty string. The tool clears such fields by omitting them, not by sending `""`.

Because of atomicity and validation, **always run `--dry-run` first** and review
the preview and the results CSV before applying for real.

---

## Safety features

- **`--dry-run`** (or `dryrun: true`): runs the full flow but never writes to SCM;
  prints a per-field preview and writes a results CSV with `status=dry-run`.
- **Pauses:** `stopfirstone`, `stopevery N`, `stoponerror` ask for confirmation
  during a real run.
- **Audit trail:** every run writes `results_<timestamp>.csv` with columns
  `id, name, position, status, changed_fields, message` — one row per rule, so you
  have a record of exactly what happened (`ok` / `skipped` / `dry-run` / `error`).

---

## License

[MIT](LICENSE) © 2026 Jordi Oliveras
