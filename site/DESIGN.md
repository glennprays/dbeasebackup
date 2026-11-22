# DBEaseBackup site design system

The public site (landing page and docs at dbease.glennprays.com) uses one
small design system. Tokens live in `styles/tokens.css`; the internal page
`/_styleguide/` renders them straight from that file.

## Reference

The only reference is the Refero "Stripe" style
(<https://styles.refero.design/style/48e5de76-05d5-4c4e-a269-c7c245b291ec>).
We borrow how it works, not how it looks.

| Taken from the reference | What we do instead, so it is not a clone |
| --- | --- |
| Near-monochrome base, exactly one accent, used only for actions, links, focus and icon strokes | Warm paper palette (`#fbfaf7`) and an oxide accent (`#b4441c`) instead of cool white and indigo |
| No shadows. Depth comes from background tint steps | Tint steps are paper → band → wash → rule; no shadow appears anywhere |
| 1px hairline rules between sections, 96px section gap, 8px base unit | Kept. Rules also run through tables and the hero ledger like ledger lines |
| Light display type (300), never bold. Tracking tightens as size grows. Tabular numerals | IBM Plex Sans instead of Söhne; JetBrains Mono for data, code and labels |
| 4px radius on buttons, inputs and cards | Kept. Tags are square 2px mono labels, not pills |
| Filled primary button always paired with an outline secondary | Kept, plus a mono copy-command block for the `docker run` line |
| Left-aligned, typography-led hero | The hero carries a product-specific backup ledger (example runs with real filename format) |

Ignored from the reference: its prompt guide gives the primary button a
9999px radius, which contradicts its own 4px rule.

## Tokens

### Color

| Token | Value | Use |
| --- | --- | --- |
| `--surface-canvas` | `#fbfaf7` | Page background |
| `--surface-band` | `#f4f1ea` | Footer, quiet sections, code blocks, sidebar |
| `--surface-wash` | `#f3e6df` | Highlighted rows, selection, active nav item |
| `--color-rule` | `#e4ded1` | Hairlines and borders |
| `--color-rule-strong` | `#cfc7b6` | Input borders, table header rule |
| `--color-ink` | `#1b1e24` | Headings and body text |
| `--color-secondary` | `#4e535c` | Lead paragraphs, short supporting text |
| `--color-muted` | `#63676f` | Captions and labels only |
| `--color-accent` | `#b4441c` | Filled buttons, links, focus ring |
| `--color-accent-hover` | `#9a3816` | Hover and pressed states |
| `--color-accent-border` | `#e3b8a6` | Outline-button border (decorative; the text carries contrast) |
| `--color-ok` / `-wash` | `#2f6b3a` / `#e9efe6` | Ledger status, success callout |
| `--color-warn` / `-wash` | `#8a5a00` / `#f5ecd9` | Ledger status, warning callout |
| `--color-fail` / `-wash` | `#a3281f` / `#f6e3df` | Ledger status, caution callout |

Status colors appear only in the ledger and docs callouts.

### Measured contrast (WCAG 2.x)

Ratios computed with the WCAG relative-luminance formula. AA body text
needs 4.5:1.

| Foreground | on canvas `#fbfaf7` | on band `#f4f1ea` | on wash `#f3e6df` |
| --- | --- | --- | --- |
| ink `#1b1e24` | 16.0:1 | 14.8:1 | 13.7:1 |
| secondary `#4e535c` | 7.4:1 | 6.9:1 | 6.3:1 |
| muted `#63676f` | 5.4:1 | 5.0:1 | 4.6:1 |
| accent `#b4441c` | 5.3:1 | 4.9:1 | 4.5:1 |
| accent-hover `#9a3816` | 6.8:1 | 6.3:1 | 5.8:1 |
| ok `#2f6b3a` | 6.1:1 | 5.7:1 | 5.2:1 |
| warn `#8a5a00` | 5.7:1 | 5.3:1 | 4.9:1 |
| fail `#a3281f` | 7.0:1 | 6.5:1 | 6.0:1 |
| code-name `#1d4f73` | 8.3:1 | 7.7:1 | 7.1:1 |
| code-number `#7a4a00` | 7.2:1 | 6.6:1 | 6.1:1 |

Other pairs: white on accent 5.5:1, white on accent-hover 7.1:1; ok on
ok-wash 5.5:1, warn on warn-wash 5.1:1, fail on fail-wash 5.9:1. The
focus ring (accent, 2px) is 5.3:1 against canvas, above the 3:1 needed for
non-text UI. The first muted grey tried (`#7a7f88`, 3.9:1) failed and was
replaced.

### Type

- IBM Plex Sans: 300 for display and headings, 400 for body and UI, 500
  for small emphasis (h4, active nav).
- JetBrains Mono: 400/500 for code, env vars, filenames and 12px
  uppercase section labels.
- Scale: 12, 14, 16, 18, 20, 22, 26, 32, 48, 56 px.
- Tracking: −0.01em up to 26px, −0.02em at 32–48px, −0.025em at 56px,
  +0.06em on uppercase mono labels.
- Body line-height 1.6 for reading; headings 1.05–1.15.
- `font-feature-settings: "tnum"` everywhere so numbers line up in tables.
- Fonts are self-hosted woff2, latin subset, `font-display: swap`. Only
  Plex 300 and 400 are preloaded. Licences: `assets/fonts/OFL-*.txt`.

### Space, shape, layout

- Spacing: 8, 16, 24, 32, 40, 48, 64, 80, 96 px (4px allowed inside dense
  UI such as the ledger and sidebar).
- Section gap 96px (64px under 720px), hairline rule between sections.
- Radius 4px on buttons, inputs, cards and code blocks; 2px on tags.
- Page max width 1240px, reading measure 68ch.

## Do

- Use the accent only for something the reader can act on.
- Separate sections with a hairline rule and whitespace.
- Keep text left-aligned.
- Pair every filled button with an outline or text-link secondary.
- Show real mechanisms: command names, file names, field names.
- Label example data as example data.

## Don't

- No shadows, gradients, glows, glass or blur effects.
- No pill buttons, no radius above 4px.
- No bold headings; weight 300 at display sizes is the identity.
- No emoji or decorative icon sets; no stock illustrations.
- No rows of three identical icon-plus-blurb cards.
- No centered marketing blocks.
- No invented metrics, logos or testimonials, and no hype copy.
- Do not use secondary or muted grey for more than a sentence of text.
