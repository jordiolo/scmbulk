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

## Safety

- `--dry-run` / `dryrun: true` previews every change and writes a results CSV
  with `status=dry-run` — nothing is written to SCM.
- `stopfirstone`, `stopevery N`, `stoponerror` pause for confirmation.
- Every run writes `results_<ts>.csv` (id, name, position, status,
  changed_fields, message).
