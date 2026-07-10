# TrendCortex UI/UX Engineering Standard

This standard applies to every user-facing TrendCortex page, component, workflow, and UI change.

## Core Rules

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
- Button labels must describe the user outcome, not backend mechanics.

## Cards And Surfaces

- Cards must use consistent spacing, radius, border opacity, background, and typography.
- Cards should frame actual content or repeated items. Do not use nested cards for basic page sections.
- Body text must be readable: comfortable line-height, sensible max width, and no dense full-width paragraphs.
- User-facing copy should be plain-language creator workflow copy, not API/provider terminology.

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
