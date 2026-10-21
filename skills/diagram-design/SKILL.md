---
name: diagram-design
description: Draws one clean, self-contained SVG diagram (architecture, sequence, state machine, database schema) from a model that is already decided. Used by Orca's Outcome Review. Never asks questions, never reads or writes files.
---

# diagram-design (Orca first-party)

You are given a `<diagram>` whose `<model>` lists the nodes and edges. The model is the truth: you only **draw** it. Do not add, drop, rename or reorder anything the model says. Orca checks the result and refuses a drawing that breaks its rules.

Work in your head: there is no tool to run. Compute the coordinates with the rules below, then write the page once.

## Page

```html
<!doctype html>
<html><head><meta charset="utf-8"><title>…</title></head>
<body style="margin:0;background:transparent">
<svg xmlns="http://www.w3.org/2000/svg" role="img" viewBox="0 0 W H" width="100%" font-family="ui-sans-serif, system-ui, sans-serif">
  <title>one line: what the diagram shows</title>
  <desc>one line: how to read it</desc>
  <defs>…marker…</defs>
  …
</svg></body></html>
```

- Exactly one `<svg>`, with a `viewBox` that is the real extent of the drawing plus a 24 px margin. `width="100%"`, no fixed height.
- Colours only through `var(--rv-…, fallback)`: text `var(--rv-fg, #1f2328)`, page `var(--rv-bg, #ffffff)`, secondary text `var(--rv-muted, #6b7280)`, lines `var(--rv-border, #d0d7de)`, and the change colours the prompt names. Never write any other colour. Draw no page background: leave the body transparent.
- Fonts: the svg's `font-family` above, `ui-monospace, monospace` for kinds, tech and protocols. Sizes: label 13 px (weight 600), caption 11 px, arrow label 11 px. Nothing smaller than 11 px.
- No `<script>`, no event attribute, no `<foreignObject>`, no `<image>`, no `<link>`, no external URL. The only `href`/`url()` allowed point at an `#id` inside the drawing (arrow markers).
- Every id starts with the prefix the prompt gives (`rv-<section>-<variant>-`). The legend is a `<g>` whose id starts with `rv-legend`.

## Boxes

- Box: `<rect rx="8">` 208 × 68 px, stroke 1.5 px, fill `var(--rv-bg, #fff)`, stroke by state (see the prompt). Label centred at the top (13 px, weight 600); under it `[kind: tech]` or the caption in 11 px monospace, `var(--rv-muted, …)`.
- A label longer than 22 characters wraps onto a second `<tspan>` line (the same words, split at a space); grow the box to 84 px high when any label in the row wraps, and use that height for the whole row.
- A node with `componentsChanged` above 0 gets a 22 px round badge on its top-right corner with that number, stroke and text in the accent colour.
- A node with a parent is drawn inside the parent: the parent is a larger rounded rect with its label at the top-left; children sit in a grid inside it with 16 px padding.

## architecture-delta (C4 container level)

1. **Columns.** Column 0: nodes nobody points at that look like people or external systems (kind contains `person`, `actor`, `external`, `system`). Last column: data stores (kind contains `database`, `db`, `store`, `queue`, `cache`, `bucket`). Every other node goes in the column one to the right of the left-most column from which an edge reaches it (longest path, at most 4 columns in the middle; if there is a cycle ignore the edge that closes it for placing only).
2. **Rows.** Inside a column keep the model's order. Column x = 24 + column × 288; row y = 24 + row × (box height + 56). Centre the shorter columns vertically against the tallest one.
3. **Arrows.** Draw an edge from the right-middle of the source box to the left-middle of the target box with a straight line. If source and target are in the same column, leave from the bottom-middle and enter the top-middle (or the reverse), bent 40 px to the right. Give the line `marker-end` pointing at a `<marker>` defined once in `<defs>` (a 10 × 10 triangle in the line's colour; define one marker per colour you use and give each a prefixed id).
4. **Edge labels.** The protocol (and label when it differs) sits at the middle of the line, centred, 11 px monospace, on a halo: `paint-order="stroke" stroke="var(--rv-bg, #fff)" stroke-width="4"`. If two labels would touch, nudge the second one 14 px down.
5. **Several edges between the same two boxes** are drawn as parallel lines 12 px apart.

## sequence

- One lane per node, left to right in the model's order, 232 px apart. The lane head is a box (208 × 44) with the label; below it a dashed vertical lifeline down to the last message.
- One horizontal arrow per message, from the source lane to the target lane, top to bottom in `order`, 56 px apart, starting 48 px under the heads. Label above the arrow, centred, with the halo. A message to the same lane is a small loop (40 × 24 px) on its right.
- `reordered` messages carry the "~" marker and, in the caption line under the label, `was step N` (N from `baseOrder`) in the muted colour.

## state-machine

- States are rounded boxes (rx 18) in a grid, at most 4 per row, 208 × 52 px, gaps 64 px. The initial state is the first one the model lists.
- Transitions are straight arrows between box edges (curved by 30 px when they would cross a box). Label on the halo.

## database-schema

- A table is a box with a header strip (label, 13 px bold) and one 22 px row per column (name left, type right in monospace). Tables in a grid, at most 3 per row, 224 px wide, gaps 72 px.
- A relation is a line from the key row of one table to the row of the other, with a small circle at the "one" end and a three-line "crow's foot" at the "many" end.

## Quality bar

- No overlap: no label crosses a box edge, no arrow crosses a box (reroute round it with one bend).
- Reading order is left to right, top to bottom; the change must be visible at a glance (state colours, marker characters and the legend), and never by colour alone.
- Keep the file small: no repeated style blocks, short ids, no comments, no decoration.
- Anything the model marks `advisory` or `unverified` is drawn thinner and lighter, never hidden.
