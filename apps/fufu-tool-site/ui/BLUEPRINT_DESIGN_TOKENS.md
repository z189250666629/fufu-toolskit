# fufu Blueprint Design token map

The unified navigation page and admin console now share one Blueprint Design layer:
drafting-grid background, compact operational surfaces, rectangular controls, and a single blue accent.

## Token families

| Layer | Token family | Purpose |
| --- | --- | --- |
| Canvas | `--blueprint-canvas`, `--blueprint-canvas-raised` | Page background and elevated drafting surface |
| Panel | `--blueprint-panel`, `--blueprint-panel-muted` | Cards, admin panels, tables, form surfaces |
| Text | `--blueprint-text-primary`, `--blueprint-text-secondary`, `--blueprint-text-muted` | Title, body, and helper hierarchy |
| Accent | `--blueprint-accent`, `--blueprint-accent-muted`, `--blueprint-accent-steel` | Primary action, metadata tags, secondary labels |
| Grid | `--blueprint-grid-line`, `--blueprint-grid-line-strong` | Blueprint grid, dividers, selected states |
| Feedback | `--blueprint-success`, `--blueprint-danger` and soft variants | Operation feedback |
| Radius | `--blueprint-radius-control`, `--blueprint-radius-panel`, `--blueprint-radius-stamp`, `--blueprint-radius-nav` | Rectangular, consistent UI geometry |

## Component primitives

| Primitive | Classes |
| --- | --- |
| Shared page/header | `blueprint-page`, `blueprint-header`, `blueprint-guide-line` |
| Top actions | `blueprint-top-actions`, `blueprint-nav-button`, `blueprint-icon-button` |
| Buttons | `blueprint-button`, `blueprint-primary-button`, `blueprint-danger-button` |
| Status/tag | `blueprint-stamp`, `blueprint-message` |
| Forms/tables | `blueprint-input`, `blueprint-textarea`, `blueprint-table` |

## HeroUI bridge

HeroUI stays as the component runtime. Its common tokens (`--background`, `--foreground`,
`--surface`, `--border`, `--separator`, `--focus`, `--field-background`, `--radius`) map to the
Blueprint Design layer so navigation and admin use the same visual contract.
