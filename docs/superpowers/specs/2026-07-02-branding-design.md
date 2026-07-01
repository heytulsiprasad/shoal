# shoal branding: banner header, animated splash, logo

**Date:** 2026-07-02
**Component:** `internal/ui`
**Status:** Approved (design)

## Summary

Integrate the user-provided `branding.go` module into the TUI: a fish-shoal
diamond logo, a multi-row banner header (replacing the current one-line header),
and an animated "living shoal" startup splash. The compact logo also opens the
first-run home screen.

## Goals

- Banner header (fish icon + block `SHOAL` wordmark + tagline) on normal-size
  terminals; degrades to a one-line header when the terminal is small.
- Animated ~2s startup splash on launch (skippable by any key), then the app.
- Compact fish logo atop the first-run home screen.

## Non-goals (YAGNI)

- No config toggle to disable branding / animation.
- No customization of the fish glyph or colors beyond the existing palette.

## Source module

`branding.go` (provided, `package ui`) supplies, with lipgloss only:
- `renderLogo(w)` / `renderLogoCompact(w)` — the fish-diamond + `s  h  o  a  l` wordmark.
- `(Model).renderHeader()` + `(Model).headerHeight()` + `(Model).noticeText()` — the banner (6 rows at ≥60×20, else a 1-row compact header pinning the toast right).
- `(Model).renderSplash(w,h,t,animate)` + `renderScene(w,sceneH,t)` — the still-logo pre-size flash (`animate=false`) and the animated shoal (`animate=true`).
- Helpers `fishRow`/`fishStyle`/`centerText`/`blockShoal`/`buildSplashFish`/`clampInt`.

It is added as `internal/ui/branding.go` verbatim except: drop the trailing
`INTEGRATION` comment block (that wiring is done for real below).

## Dependencies to satisfy

- **Style `st.Dim`** does not exist (the palette has a `Dim` color; the `Styles`
  struct exposes `Logo/Tag/Accent/Faint/Notice/Bad` but not `Dim`). Add a `Dim`
  field to the `Styles` struct and `Dim: s().Foreground(p.Dim)` to the builder.
  It drives the logo's accent→dim→faint depth ramp (`fishStyle` codes 3→2→1).
- Everything else branding.go needs already exists: `st.Logo/Accent/Faint/Tag/Notice/Bad`, `glyphDone`/`glyphErr`, builtin `max`, and `truncate`.

## Collision

`internal/ui/view.go` already defines `func (m Model) renderHeader()` (the
one-line header). Delete it; branding.go's banner `renderHeader` + `noticeText`
replace it. No other symbol in branding.go collides (verified: `renderLogo`,
`centerText`, `fishRow`, `blockShoal`, `clampInt`, `noticeText`, `renderSplash`,
`renderScene`, `headerHeight` are all new).

## Model wiring (the six hooks)

- **Fields:** `booting bool`, `frame int` on `Model`.
- **Constructor:** `NewWithConfig` sets `booting: true`.
- **Timing:** `const frameInterval = 55 * time.Millisecond`, `const splashFrames = 36` (~2s); `func (m Model) splashT() float64 { return float64(m.frame) * frameInterval.Seconds() }`; `type frameMsg struct{}`; `func frameCmd() tea.Cmd { return tea.Tick(frameInterval, func(time.Time) tea.Msg { return frameMsg{} }) }`.
- **Init:** add `frameCmd()` to the existing `tea.Batch(...)`.
- **Update:** add
  ```go
  case frameMsg:
      if !m.booting { return m, nil }
      m.frame++
      if m.frame >= splashFrames { m.booting = false; return m, nil }
      return m, frameCmd()
  ```
- **handleKey:** immediately after the `ctrl+c` quit check, `if m.booting { m.booting = false; return m, nil }` (any key skips the splash; ctrl+c still quits first).

## View

Replace the top of `View()`:
```go
if !m.ready {
    return m.renderSplash(80, 24, 0, false) // pre-size flash: still logo
}
if m.booting {
    return m.renderSplash(m.width, m.height, m.splashT(), true)
}
if m.showHelp {
    return m.helpView()
}
header := m.renderHeader()
rule   := st.Rule.Render(strings.Repeat("─", max(1, m.width)))
bodyH  := max(3, m.height-m.headerHeight()-3) // header (up to 6) + 2 rules + footer
body   := m.renderBody(bodyH)
footer := m.renderFooter()
return strings.Join([]string{header, rule, body, rule, footer}, "\n")
```

## Home screen

In `renderHome`, replace the text line `st.Logo.Render("Welcome to shoal")` with
`renderLogoCompact(w)` (keep the "A calm BitTorrent client…" blurb and the
HOW IT WORKS / START HERE content below).

## Test reconciliation (two integration snags)

1. **`ready()` test helper must clear `booting`.** With `booting: true` by
   default, every existing test that calls `ready(New(...))` then `View()` would
   render the splash instead of the app. `ready()` means "app is up and settled",
   so it sets `m.booting = false` (after the `WindowSizeMsg`). This unblocks all
   existing ready()-based tests.
2. **`TestHomeShownBeforeFirstSearch`** asserts the literal `"Welcome to shoal"`,
   which the compact logo replaces. Update it to assert the branding instead
   (e.g. the `s  h  o  a  l` wordmark and/or the `HOW IT WORKS` section).

`TestNewModelDefaults` (pre-`ready`) asserts `View()` contains `"starting shoal"`;
the new `!ready` path returns the static splash which still contains
`"starting shoal…"`, so it continues to pass — no change needed.

## Testing (TDD)

- `renderHeader` at ≥60×20 contains the block wordmark (a `█` run) and the tagline; `headerHeight()` returns 6; at a small size (e.g. width 40) it returns 1 and `renderHeader` is a single line containing `shoal`.
- `renderLogo(w)` and `renderLogoCompact(w)` contain the `s  h  o  a  l` wordmark.
- `renderSplash(w,h,0,false)` (static) contains `"starting shoal…"` and the wordmark; `renderScene(w, 3, t)` returns 3 lines.
- Model: `New(...)` has `booting == true`; a `frameMsg` at `frame == splashFrames-1` settles `booting=false`; any key while `booting` clears it (and returns no further command beyond consuming the key); `ready()` yields `booting == false`.
- `renderHome` output contains the compact-logo wordmark (updated `TestHomeShownBeforeFirstSearch`).

## Error handling / edge cases

- Small terminals: `headerHeight()==1` compact header keeps 80×24 and narrower usable; `renderSplash` sizes the scene to `max(3, h-3)`.
- The splash ticker runs only while `booting`; `frameMsg` is a no-op once settled, so no runaway timers.
- All rendering degrades under NO_COLOR/256 via the existing palette (the diamond formation carries the logo without color).

## Files touched

- Create: `internal/ui/branding.go` (provided module, minus the trailing integration comment).
- Modify: `internal/ui/theme.go` (add `st.Dim`).
- Modify: `internal/ui/view.go` (delete old `renderHeader`; `View` splash branches + `headerHeight`-based `bodyH`; `renderHome` compact logo).
- Modify: `internal/ui/model.go` (booting/frame fields, constructor, `frameCmd`/`frameMsg`/`splashT`, `Init`, `Update`, `handleKey`).
- Modify: `internal/ui/model_test.go` (update `ready()` helper; `TestHomeShownBeforeFirstSearch`; new booting tests).
- Modify: `internal/ui/view_test.go` (header/logo/splash tests).
