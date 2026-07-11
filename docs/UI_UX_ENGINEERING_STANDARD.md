# TrendCortex UI/UX Engineering Standard

This standard applies to every user-facing TrendCortex page, component, workflow, and UI change.

## Core Rules

- Every UI change must be visually inspected in Codex's internal browser before approval.
- Never approve a layout based only on source code review, build success, screenshots from another environment, or tests.
- Use real product states only. Do not add fake data, fake metrics, fake accounts, or fake notifications for visual completeness.
- Use an 8px spacing system and shared design tokens/components instead of one-off page styling.
- Keep desktop content centered inside a controlled maximum width.
- Avoid pure black and pure white on major surfaces.
- Use one restrained brand accent plus separate semantic colors for success, warning, error, and information.
- Prefer typography weights 300-600. Avoid italics and excessive bold text in the regular interface.
- Every page must have one obvious primary action.
- Empty, loading, disabled, success, and error states must be intentionally designed.
- Responsive behavior must be tested at desktop, tablet, and mobile widths.
- Tablet layouts must retain vertical navigation through a compact sidebar or accessible drawer.
- Never replace the sidebar with a crowded horizontal navigation bar.
- Every responsive breakpoint must be visually tested in Codex's internal browser.
- Accessibility contrast and visible keyboard focus states are required.
- Visual polish must never introduce fake functionality or change API contracts.
- User-facing pages must never show raw JSON, raw IDs, backend diagnostics, stack traces, provider payloads, or developer wording.
- Creator-facing UI must not expose backend implementation details.
- Provider diagnostics belong in development/admin tooling, not normal creator Settings.
- Platform connection management belongs on Connections.
- Settings must contain creator-facing preferences only and must not duplicate platform account connection management.
- Settings must not claim server-side persistence unless a real backend persistence path exists.
- Shared platform selectors must be implemented as reusable components.
- Raw backend enum values must not appear in the UI.
- Google Trends is a research source, not a social publishing platform.
- Coming Soon states must be honest shells only: no fake metrics, fake activity, or non-functional primary actions.
- Build success alone does not count as visual QA.
- Every page must have one clear user goal. Secondary information should support that goal, not compete with it.
- Every action button must visibly relate to the content it affects. Section actions belong inside or directly beside the affected section.
- Use progressive disclosure. Advanced, debug, diagnostic, and evidence details must live inside collapsed `Advanced details` or `Advanced evidence data` sections.
- Red is only for real errors that require attention. Warnings, missing connections, or setup states use neutral or muted styling unless the user is blocked.
- `Not configured`, `not connected`, and similar setup states must be neutral, calm, and explanatory.

## Button System

- Primary buttons are for the main workflow CTA on the screen.
- Secondary buttons are for common supporting actions.
- Ghost buttons are for low-emphasis navigation, disclosure, or optional actions.
- Disabled buttons must clearly look unavailable and must not imply failure.
- Disabled controls must explain why they are disabled.
- Primary actions must appear in a dedicated and predictable action area.
- Button labels must describe the user outcome, not backend mechanics.
- Buttons must use semantic `<button>` elements for actions and links only for navigation.
- Touch targets should be approximately 44px tall on mobile.
- Button groups must wrap or stack before labels clip or controls become unreachable.

## Cards And Surfaces

- Cards must use consistent spacing, radius, border opacity, background, and typography.
- Cards should frame actual content or repeated items. Do not use nested cards for basic page sections.
- Avoid more than three visible surface elevation levels: page background, section/card surface, and interactive or data-row surface.
- Avoid oversized status rows.
- Body text must be readable: comfortable line-height, sensible max width, and no dense full-width paragraphs.
- User-facing copy should be plain-language creator workflow copy, not API/provider terminology.
- Raw backend enum values must be translated into user-friendly UI labels.
- Logos must be evaluated at actual sidebar and favicon sizes.
- The right workspace should be centered inside a controlled content width, normally 1320px-1440px.
- Data-heavy pages may use the wider end of the workspace range, but forms must not create very long text fields.
- Card hierarchy must distinguish static information, interactive selections, form panels, status cards, and empty states.
- Interactive cards may use subtle hover feedback only when the whole card is actionable.
- Do not use large shadows, repeated backdrop blur, or decorative gradients to compensate for weak hierarchy.

## Forms And Controls

- Every form control must have a visible label or an accessible name.
- Short text belongs in `<input>`, long text in `<textarea>`, and option sets in `<select>`, radio cards, checkboxes, segmented controls, or tabs as appropriate.
- Field labels, helper text, values, and validation messages must not collide at tablet or mobile widths.
- Textareas must remain usable on mobile and at 125% and 150% browser zoom.
- Dropdown arrows and selected values must stay aligned at all inspected breakpoints.
- Focus states must be visible on buttons, links, tabs, fields, cards, and disclosure controls.

## Semantic Colour

- Cyan/teal is the brand accent for primary actions, active tabs, selected states, focus, and key workflow affordances.
- Green is for connected, saved, ready, and successful states.
- Amber is for setup needed, unavailable-but-recoverable, warnings, and unsaved work.
- Red/pink is only for failed, destructive, or genuine error states.
- Blue/cyan is for informational or loading states.
- Neutral grey is for coming soon, disabled, unavailable by design, and not connected.
- Colour must never be the only indicator of state; labels, icons, and copy must also communicate the state.

## System States

- Empty, loading, error, success, unavailable, account-not-connected, provider-not-configured, and coming-soon states must use shared spacing, icon treatment, headings, explanation copy, and semantic colour.
- State headings must be product-facing and honest.
- State descriptions should be short and explain what is happening or what is missing.
- A next action should appear only when a real action is available.
- Do not show unexplained acronym placeholders such as `ER`, `NC`, `DEV`, or similar large letter tiles.

## Motion

- Page transitions may animate only the right workspace content, never the sidebar.
- Route changes should render immediately and use short, subtle opacity/position motion.
- Animation must not cause layout shifts, text blur, horizontal overflow, slow navigation, hydration issues, or repeated replay on trivial state updates.
- Avoid full-screen fades and dark overlays for route transitions.
- Respect `prefers-reduced-motion`; nonessential motion must be disabled or reduced.
- Prefer CSS transitions and keyframes over adding animation libraries.

## Responsive Workspace

- Desktop pages should use balanced grids and shared container edges for page titles, banners, tabs, cards, and forms.
- Tablet layouts may keep two columns only where content fits naturally; otherwise collapse to one column.
- Mobile layouts must be single-column, readable, and free of horizontal overflow.
- Tabs and segmented controls must scroll or wrap without clipping labels.
- Page title and action areas must wrap naturally.
- Cards, banners, and form rows must stack before text or controls overlap.
- Main content must not disappear beneath the header or drawer.

## Workflow Model

The main TrendCortex workflow is:

Trend Finder -> Generate Script -> Script Studio -> Use in Clip Generator -> Download/Publish.

UI changes must preserve this CTA flow. A user should always understand where they are and what the next useful step is.

## Product Navigation

The main sidebar information architecture is:

- Dashboard
- Research: Trending Keywords, Platform Trends, Video Analyzer, Channel Analyzer, Niche Finder
- Content: Script Studio, Clip Generator, Voice Studio, Thumbnail Studio, Assets
- Publishing: Connections, Calendar, Analytics
- Settings

Settings subsections belong inside the Settings workspace, not as main sidebar items. Creator settings, platform connections, and developer diagnostics must stay separated:

- Creator settings: Workspace, AI, Publishing defaults, Branding, Billing, Team.
- Platform account and OAuth setup: Connections.
- Provider diagnostics, environment readiness, and implementation details: `/developer/system-status`, hidden from normal navigation.

## Evidence And Grounding

- Evidence must be parsed into clear sections such as source summary, evidence sources, performance indicators, related videos, related channels, keywords, and limitations.
- Unknown or advanced evidence fields must be hidden inside a collapsed advanced details section.
- Evidence limitations must be clear but not alarming. Prefer neutral language such as: `Public metadata only. Exact RPM/search ranking requires authorized analytics.`

## Pre-Commit UI Checklist

Before committing any UI change, Codex must visually inspect desktop, tablet, and mobile and verify:

- No horizontal overflow.
- No raw JSON, raw IDs, backend diagnostics, or developer wording are visible by default.
- No fake data is introduced.
- No broken CTA flow.
- No console errors.
- A successful build does not count as visual QA.
- Code checks alone are never sufficient for UI approval.
- Inspect representative routes in the internal browser, interact with tabs, fields, dropdowns, buttons, hover states where possible, and keyboard focus.
- Verify 100%, 125%, and 150% browser zoom for affected layouts when the change touches workspace structure, cards, forms, tabs, or responsive behavior.
- Screenshot representative before/after pages when the task requires visual review.
