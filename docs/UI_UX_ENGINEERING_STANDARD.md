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

## Cards And Surfaces

- Cards must use consistent spacing, radius, border opacity, background, and typography.
- Cards should frame actual content or repeated items. Do not use nested cards for basic page sections.
- Avoid more than three visible surface elevation levels: page background, section/card surface, and interactive or data-row surface.
- Avoid oversized status rows.
- Body text must be readable: comfortable line-height, sensible max width, and no dense full-width paragraphs.
- User-facing copy should be plain-language creator workflow copy, not API/provider terminology.
- Raw backend enum values must be translated into user-friendly UI labels.
- Logos must be evaluated at actual sidebar and favicon sizes.

## Workflow Model

The main TrendCortex workflow is:

Trend Finder -> Generate Script -> Script Studio -> Use in Clip Generator -> Download/Publish.

UI changes must preserve this CTA flow. A user should always understand where they are and what the next useful step is.

## Evidence And Grounding

- Evidence must be parsed into clear sections such as source summary, evidence sources, performance signals, related videos, related channels, keywords, and limitations.
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
