# scmbulk

CLI to bulk-change Palo Alto SCM (Strata Cloud Manager) security rules.

## Build

```bash
go build -o scmbulk .
```

## Configure

Copy `config.yaml.example` to `config.yaml` and fill in the `scm` block
(client_id, client_secret, tsg_id, folder). `config.yaml` is gitignored.

## Mode A — edit a CSV

```bash
./scmbulk download --position both            # writes rules_<folder>_<ts>.csv
# edit the CSV in Excel: change cells, keep only the rows you want to change
./scmbulk apply --file rules_edited.csv --dry-run   # preview: field old -> new
./scmbulk apply --file rules_edited.csv             # apply for real
```

List fields (source, destination, application, service, tag, from, to, ...) use
`;` inside a cell. `any` is kept literal. `profile_setting` is shown as
`group:<name>`. Only cells you actually change are written back.

## Mode B — declarative filter + change

Fill `selection` and `change` in `config.yaml`, then:

```bash
./scmbulk apply --select --dry-run
./scmbulk apply --select
```

`change.set/add/remove` values accept Go templates rendered per rule with the
rule's own fields as context and helpers `has`, `contains`, `lower`, `upper`,
`replace`, `join`, `split`. Example:

```yaml
change:
  set:
    action: '{{ if (eq .action "allow") }}deny{{ else }}drop{{ end }}'
  add:
    tag: ["reviewed"]
```

## Editable fields

Any of the columns the tool serializes can be changed; the write is a full
GET → modify → PUT round-trip, so fields you don't touch are preserved:

```
name, description, policy_type, action, from, to, source, source_hip,
destination, destination_hip, source_user, application, service, category,
tag, log_setting, log_start, log_end, disabled, negate_source,
negate_destination, profile_setting, schedule, devices
```

Some fields exist on rules but are **preserved, not editable** via the CSV
because their value is a nested object that a flat cell cannot represent
faithfully: `allow_url_category`, `allow_web_application`, `log_settings`,
`security_settings`. They are sent back unchanged on every write.

Value formats:
- List fields (`from, to, source, destination, source_user, application,
  service, category, tag`) use `;` inside the cell. `any` is kept literal.
- Boolean fields (`disabled, negate_source, negate_destination, log_start,
  log_end`) are `true`/`false` (case-insensitive on input).
- `profile_setting` is shown/edited as `group:<name>` (only the group form is
  supported; the `profiles` form is preserved but not editable).
- Emptying a scalar cell (e.g. `description`) clears that field (the key is
  omitted from the PUT). Clearing `profile_setting` (empty cell) drops the group.

Fields that are NOT in the column list above cannot be edited via the CSV, but
they are never lost — the round-trip PUT sends the whole rule back unchanged for
those fields.

## SCM API behavior to know

Learned and verified against a live tenant:

- **The PUT is full-replace.** The tool always sends the complete rule object
  (only `id`/`folder` are stripped), which is why untouched fields survive. A
  field omitted from the body is cleared server-side — this is how "clear a
  field" works.
- **The PUT is atomic.** If any part of a rule is invalid, the whole update is
  rejected and nothing in that rule changes; the error is recorded per rule in
  the results CSV.
- **Reference fields must point to existing objects.** `tag`, and likewise
  address/service/application/zone/log-setting/profile-group values, must
  already exist in the tenant/folder. Assigning an arbitrary name fails with
  `INVALID_REFERENCE` (e.g. `tag 'foo' is not a valid reference`). Create the
  object in SCM first.
- **Some fields reject empty values.** For example `description` cannot be an
  empty string (`"not allowed to be empty"`). The tool handles a cleared scalar
  cell by omitting the field (which clears it) rather than sending `""`.

Because of atomicity and validation, always run `--dry-run` first and review the
preview and the results CSV before applying for real.

## Safety

- `--dry-run` / `dryrun: true` previews every change and writes a results CSV
  with `status=dry-run` — nothing is written to SCM.
- `stopfirstone`, `stopevery N`, `stoponerror` pause for confirmation.
- Every run writes `results_<ts>.csv` (id, name, position, status,
  changed_fields, message).
