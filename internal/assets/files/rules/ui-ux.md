# Rules — UI / UX / Design

Frontend is not "the part we bolt on at the end". The UI is where your
software meets a human being — it determines whether the feature you built
is actually usable, accessible, and pleasant. Every feature with a visible
surface MUST be evaluated against this file before declaring Ready.

## Principle

- **Clarity over cleverness.** The user should understand the interface without a manual.
- **Consistency over novelty.** Familiar patterns beat clever reinventions unless you have data.
- **Feedback is mandatory.** Every action gets a response: loading, success, error. No black holes.
- **Every state has a design.** Loading, empty, error, success, forbidden, offline, permission-denied — not afterthoughts.
- **Design for the worst case first.** Narrow viewport, poor network, keyboard-only, screen reader, 200% zoom, disabled JS, low contrast.
- **Accessibility is non-negotiable.** WCAG 2.2 AA is the floor, not the ceiling.
- **Performance is UX.** A slow UI is a broken UI.

## Interview — questions to answer before any design

For every feature with a visible surface, the Architect/Discover phase MUST
resolve these via `AskUserQuestion`:

### Product / UX

- Who uses this? What job are they trying to get done? What emotion should they feel after?
- What is the **primary action** on this surface? What is secondary?
- What success looks like, measurably (task completion rate, time on task, error rate).
- What do users see **before** this feature exists? What is the journey in and out?
- Empty state: first-time user, nothing configured — what do they see?
- Error states: validation, network, permission, server — each gets a design.
- Edge-case data: very long names, 0 items, 10000 items, RTL languages, pasted content with whitespace/emojis.

### Visual

- Brand / design system — tokens, components, tone of voice.
- Color palette (primary/secondary/semantic) — sourced from tokens, not invented.
- Typography scale (display, heading, body, caption) and hierarchy rules.
- Spacing system (4pt / 8pt grid) and layout grid.
- Iconography source (library? custom set?).
- Illustration / imagery style.
- Motion language — allowed easings/durations, when to animate, when not to.

### Platform & reach

- Target viewports: mobile-first? desktop-first? both? breakpoints?
- Supported browsers (evergreen baseline, or wider list?).
- Dark mode? Light mode? System-synced? Per-user?
- Internationalization: which locales, RTL required?
- Offline behavior required?
- Keyboard-only use required (enterprise, finance, accessibility mandate)?

## UX patterns — defaults

### State coverage (every screen)

For every view, define all of:

- **Loading** — skeleton, shimmer, spinner, or progress bar. Never a frozen white screen.
- **Empty** — first-time message + CTA. Not "no results".
- **Partial empty** — list filtered to zero results; include "clear filters".
- **Error** — specific, actionable, recoverable. Never "an error occurred".
- **Success** — explicit confirmation for destructive or long-running actions.
- **Offline** — cache hits, read-only mode, or an explicit offline banner.
- **Forbidden / unauthorized** — a real page, not a blank 403.

### Forms

- **One column layout** for forms unless parallel fields make semantic sense.
- Every input has a **visible label**. Placeholder ≠ label.
- Errors appear **below the field** that caused them, in plain language, as soon as the user leaves the field.
- **Required fields are marked**; optional is the default assumption only if most fields are required.
- **Inline validation** for fixable problems; submit-time validation for server-only checks.
- Never clear a form on error — preserve the user's input.
- Submit button: disabled while submitting, labeled with the action (not "Submit"), never double-clickable.
- **Autofocus the first empty field** on first render only; never steal focus mid-interaction.
- Support browser autofill (`autocomplete="..."` attributes).
- Use the right `inputmode`/`type` (email, tel, url, number) for mobile keyboards.

### Lists / tables / data

- **Pagination or infinite scroll** — pick one per context. Cursor-based.
- Column count responsive: collapse on narrow viewports into cards or stacked rows, don't horizontal-scroll blindly.
- **Empty lists** have a message + CTA.
- Sortable columns show the current sort; clicking the same column toggles.
- Filters show their current state; one-click reset.
- Bulk actions appear when items are selected; never as always-visible toolbars that fight primary actions.

### Modals / dialogs

- Open modals trap focus; Esc closes; clicking outside closes (unless destructive).
- Primary action on the **right**; secondary/cancel on the left (or above, on mobile).
- Confirm on destructive actions; never confirm on reversible, non-destructive ones.
- Never stack modals.

### Navigation

- Active item is visually distinct (not only color; add weight/underline/marker).
- Breadcrumbs where depth > 2.
- Back behavior does what the browser back button does.
- Bottom nav on mobile; side nav on desktop; do not mix.

### Feedback & motion

- Optimistic UI for low-risk actions (likes, bookmarks, reorders) with rollback on error.
- Toast / inline banner — pick one convention per app.
- Motion has a **purpose**: draw attention, hint at origin/destination, confirm a state change.
- Durations: 100–300 ms for micro (hover, focus), 200–500 ms for state changes, 500 ms+ only for meaningful transitions.
- Easing: `ease-out` for entering, `ease-in` for exiting, `ease-in-out` for movement.
- Respect `prefers-reduced-motion: reduce` — disable transform/opacity transitions.

## Accessibility (WCAG 2.2 AA minimum)

- **Semantic HTML** over ARIA. `<button>` over `<div onClick>`. ARIA is a patch, not a default.
- **Keyboard**: every interactive element is reachable with Tab, operable with Enter/Space, in a logical order. Visible focus ring (never `outline: none` without replacement).
- **Focus management**: route changes move focus to the new page's heading; modal opens move focus into the modal; modal closes return focus to the trigger.
- **Contrast**: text ≥ 4.5:1 (≥ 3:1 for 18pt+ or bold 14pt+); UI controls ≥ 3:1.
- **Labels**: every form control, icon-only button, and landmark has an accessible name.
- **Live regions** for async updates (errors, toasts, search results): `aria-live="polite"` / `assertive`.
- **Headings** in order (no skipping levels). One `<h1>` per page.
- **Landmarks** in use: `header`, `nav`, `main`, `aside`, `footer` — assistive tech depends on them.
- **Alt text**: descriptive for content images; `alt=""` for decorative. No "image of …" prefix.
- **Forms**: `<label for>`, `aria-describedby` for hints, `aria-invalid` on error.
- **Color is never the only signal** (error = red + icon + text).
- **Zoom to 200%** without loss of content or function. **Reflow at 320 CSS px**.
- **Motion**: honor `prefers-reduced-motion`; avoid parallax, auto-play video with motion.
- **Target size**: touch targets ≥ 24×24 CSS px (WCAG 2.2), ideal 44×44.
- **Captions / transcripts** for audio/video.
- **Test with a screen reader** (VoiceOver, NVDA, TalkBack) at least once per feature.

Automated tooling is necessary but not sufficient:
- `@axe-core/playwright`, `jest-axe`, `lighthouse-ci` in CI — zero violations on main flows.
- Keyboard-only smoke test in the e2e suite.

## Responsive design

- **Mobile-first** styles; add complexity at wider breakpoints.
- Breakpoints tied to **content**, not to specific devices: use `rem`/`em` in media queries.
- Test 320, 375, 768, 1024, 1440, 1920 CSS px.
- Touch targets ≥ 44×44 px on touch viewports; spacing ≥ 8 px between interactive items.
- **No horizontal scroll** on body at any viewport (except intentional data tables).
- Input zoom on iOS: font-size ≥ 16 px on inputs.
- Respect safe areas: `env(safe-area-inset-*)` on mobile.

## Typography

- Use a **type scale** (e.g. 12/14/16/18/20/24/32/48). No one-off sizes.
- Body text ≥ 16 px. Smaller text reserved for metadata.
- Line-height: 1.4–1.6 for body, 1.1–1.3 for headings.
- **Line length** 50–75 characters. Enforce with `max-width` or `ch` units.
- Numerals: tabular numerals in tables/columns.
- One typeface family for body, at most two including display.

## Color

- Use **tokens** (`--color-primary`, `semantic.success.fg`). Never hard-coded hex in components.
- Semantic tokens (`fg.default`, `bg.surface`, `border.subtle`, `feedback.danger`) layer above raw palette.
- Meaning is consistent across the app (red = destructive/error, always).
- Dark mode is a separate token set, not an opacity trick.
- Check contrast for every token pair that can render text — in both themes.

## Spacing & layout

- **4pt or 8pt grid.** Every margin/padding/gap is a grid multiple.
- Use layout primitives (`Stack`, `Inline`, `Grid`) over one-off margins.
- **Vertical rhythm**: space between sections ≥ 2× space between items within a section.
- Avoid absolute positioning unless the element is truly floating (toasts, modals).
- Prefer CSS Grid / Flexbox; use `gap` over margins for spacing between items.

## Components

### API design

- **Composition over configuration.** `<Card><Card.Header/><Card.Body/></Card>` beats `<Card header={...} body={...} footer={...} />`.
- **Variants** (`variant="primary"`) for discrete visual modes; never boolean-prop explosion (`isPrimary`, `isDanger`, `isLarge`).
- **Slots** for advanced customization; do not expose className for styling hooks where a variant is enough.
- **Uncontrolled by default**, controlled when needed. Support both where it makes sense.
- **No layout inside leaf components.** Components are visually neutral; parents arrange them.
- Forward refs where the underlying element would accept them.
- Respect disabled / readonly / loading states uniformly.

### Design system

- There **is** a design system. Even a tiny one. Document tokens, primitives, patterns.
- **Don't fork components locally.** Fix in the system.
- **Storybook** (or equivalent) documents every primitive with all variants, all states.
- Component additions reviewed for duplication against existing primitives.

## Content & voice

- **Use the product's voice** (concise, helpful, neutral — whatever the brand defines).
- **Specific error messages**: "Password must be at least 12 characters" > "Invalid input".
- **Action-oriented buttons**: "Send invitation" > "Submit". Never "OK" for a destructive action.
- **Sentence case** for UI copy (unless the brand system says otherwise). Avoid ALL CAPS.
- **No jargon** unless the user is a specialist in that domain.
- **No lying**: "Saved!" appears only if it actually saved.
- Content strings live in an **i18n layer** from day 1, even if you ship one locale.

## Internationalization

- Extract strings to an i18n system from the start.
- **Never concatenate strings** (`"Hello " + name + ", you have " + n + " items"`). Use interpolation with full sentence templates per locale.
- Handle **plurals** with ICU message format.
- Dates, numbers, currency: locale-aware formatters (`Intl.*`, native libs), never hand-rolled.
- **RTL**: test at least one RTL locale. Use logical CSS properties (`margin-inline-start`, not `margin-left`).
- Allow for **text expansion**: German can be 30-60% longer; Japanese can be denser.

## Performance UX

(cross-ref `performance.md`)

- **Perceived performance beats raw speed.** Skeletons, progressive loading, optimistic UI.
- **First meaningful content** in < 2.5 s on mid-tier mobile + 4G (LCP).
- **Interaction latency** < 200 ms (INP).
- **Bundle budget per route** set and enforced in CI.
- Images: AVIF/WebP, `srcset`, `loading="lazy"`, explicit dimensions.
- Fonts: subset, preload critical, `font-display: swap`.
- No layout shift (CLS ≤ 0.1) — reserve space for images/ads/embeds.

## Testing UI

(cross-ref `testing.md`)

- **Unit tests** on component logic (rendering branches, event handlers).
- **Interaction tests** (Testing Library, Playwright Component) for behavior that depends on DOM.
- **Visual regression** (Playwright screenshots, Chromatic, Percy) on components and pages.
- **Accessibility tests** automated (`jest-axe`, `@axe-core/playwright`) on every component & page.
- **Keyboard-only test** on at least one critical flow.
- **RTL / locale test** on at least one critical flow.
- **Real-device test** at least once per release for mobile.

## Design handoff

Every UI feature's Design spec (`02-design.md`) MUST include:

- [ ] **Wireframes or low-fi mockups** (linked). Include the empty, loading, error, success, and forbidden states.
- [ ] **User flow diagram** for multi-step journeys.
- [ ] **Responsive plan**: mobile / tablet / desktop behavior per major breakpoint.
- [ ] **State machine**: every state listed with triggers and transitions.
- [ ] **Component inventory**: which existing components are reused, which need to be added to the system.
- [ ] **Token usage**: colors, spacing, typography tokens pulled from the system.
- [ ] **A11y plan**: landmarks, focus order, keyboard interactions, ARIA where needed.
- [ ] **Motion plan**: what animates, why, duration/easing, reduced-motion fallback.
- [ ] **Content**: exact copy for all strings, placeholders, error messages, empty states.
- [ ] **i18n considerations**: dynamic content, plurals, RTL.
- [ ] **Edge cases**: long text, zero data, huge data, invalid data, slow network, offline.
- [ ] **Analytics events** emitted and why.
- [ ] **Performance budget** for the route (bundle, LCP, INP).

Missing sections → Design is **not Ready**.

## Review rubric (frontend PR)

For any PR touching UI:

- [ ] All documented states rendered (empty, loading, error, success).
- [ ] Keyboard-only flow completes (no traps, logical order, visible focus).
- [ ] Screen-reader check at least once (VoiceOver / NVDA).
- [ ] Contrast checked in both light and dark modes.
- [ ] Responsive at 320, 768, 1440 CSS px.
- [ ] No layout shift (CLS).
- [ ] No network calls on keystroke without debounce.
- [ ] No `any` on component props. Strict prop types.
- [ ] No hard-coded colors/spacing — tokens used.
- [ ] No string concatenation for user-facing text — i18n layer used.
- [ ] `loading`, `disabled`, `error` states uniform with the rest of the app.
- [ ] Motion respects `prefers-reduced-motion`.
- [ ] Bundle budget met.
- [ ] Visual regression diff reviewed.
- [ ] Automated a11y test passes.

## Forbidden

- Shipping a UI with only the happy path — loading/empty/error states missing or stubbed.
- `<div onClick>` where a `<button>` or `<a>` fits.
- Hard-coded hex colors, pixel values, or font stacks inside components (must come from tokens).
- `outline: none` / `box-shadow: none` on focus without a replacement ring.
- Placeholder text used as the only label.
- Icon-only buttons without `aria-label`.
- Error messages like "Something went wrong" in production.
- Auto-playing video with sound.
- Content layout shift after load ("jumping" UI).
- Blocking the main thread for > 50 ms (long task).
- `window.alert` / `confirm` / `prompt` for product UX.
- Concatenating user-facing strings instead of using i18n interpolation.
- Locale-specific date/number formatting hand-rolled.
- Custom dropdowns, checkboxes, radios without keyboard + screen-reader parity with native.
- Shipping without a keyboard-only smoke test on critical flows.
- Components that style their own margins (margin belongs to the layout, not the component).
- Adding a component that already exists in the design system under a different name.
