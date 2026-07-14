# Welcome Design QA

## Comparison Target

- Source visual truth: `C:/Users/Administrator/Downloads/QQ20260713-205016.mp4`
- Extracted source evidence: `C:/Users/Administrator/.codex/visualizations/2026/07/13/019f5b89-8a1f-7ea3-b11d-6f272abb6e68/welcome-reference/reference-contact-sheet.jpg`
- Implementation URL: `http://127.0.0.1:4173/welcome`
- Primary implementation screenshot: `C:/Users/Administrator/.codex/visualizations/2026/07/13/019f5b89-8a1f-7ea3-b11d-6f272abb6e68/welcome-reference/implementation-desktop-rich-v2.png`
- Mobile implementation screenshot: `C:/Users/Administrator/.codex/visualizations/2026/07/13/019f5b89-8a1f-7ea3-b11d-6f272abb6e68/welcome-reference/implementation-mobile-rich-v2.png`
- Role-view screenshots: `implementation-desktop-role-student-v2.png`, `implementation-desktop-role-teacher-v2.png`, and `implementation-mobile-role-preview-v2.png` in the same evidence directory.
- Journey screenshots: `implementation-mobile-journey-v2.png` and `implementation-mobile-journey-dark-v2.png` in the same evidence directory.
- Combined comparison evidence: `C:/Users/Administrator/.codex/visualizations/2026/07/13/019f5b89-8a1f-7ea3-b11d-6f272abb6e68/welcome-reference/motion-fidelity-comparison.jpg`
- Viewports: 1280 x 720 desktop and 390 x 844 mobile.
- State: public welcome route, light theme; dark theme also checked on mobile.

The reference is animation-only by user instruction. Its fantasy imagery, typography, copy, and palette are intentionally excluded from the visual contract. The fidelity target is the motion grammar: an authored first scene, scale-led entry into an immersive scene, layered content arrival, horizontal scene handoff, and a clean transition into the next page band.

## Full-View Comparison Evidence

- The implementation preserves the reference's title-led opening while replacing fantasy destination cards with a real product workspace.
- The title now uses a four-character staggered 3D entrance, a restrained project-blue/project-violet highlight on `智学`, and converging guide lines without changing the product name or palette.
- The initial product workspace grows from a partial first-viewport preview into a viewport-dominant sticky scene.
- AI, knowledge graph, and learning path workspaces hand off horizontally with overlapping crossfades, matching the reference's foreground/background separation and scene replacement rhythm.
- The final scene releases into the learning-loop band without a blank frame or scroll trap.
- The page now continues into a functional student/teacher role switch and an open timeline covering pre-class, active learning, and review, so the welcome surface explains a complete product journey rather than ending after the motion demo.
- Desktop and mobile keep the same story order, interaction labels, theme tokens, and CTA hierarchy.

## Focused Region Comparison Evidence

- Intro composition: `implementation-desktop-final.png` and `implementation-mobile-initial-final.png` verify brand hierarchy, CTA spacing, next-scene visibility, and responsive wrapping.
- AI stage: `implementation-desktop-tutor-fixed.png` and `implementation-mobile-tutor.png` verify product-window scale, content legibility, and stage navigation.
- Knowledge stage: `implementation-desktop-graph.png` and `implementation-mobile-graph-final.png` verify horizontal handoff, selected-state feedback, and mobile content reduction.
- Path stage: `implementation-desktop-path-final.png` and `implementation-mobile-path-clean.png` verify the final scene, inactive-layer removal, and mobile secondary-panel suppression.
- Role views: `implementation-desktop-role-student-v2.png` and `implementation-desktop-role-teacher-v2.png` verify selected states, content replacement, open feature lists, and role-specific product previews.
- Mobile product view: `implementation-mobile-role-preview-v2.png` verifies the 334px-wide collapsed student workspace with no horizontal overflow.
- Learning stages: `implementation-mobile-journey-v2.png` and `implementation-mobile-journey-dark-v2.png` verify the vertical mobile timeline, progressive reveal, and dark-theme contrast.
- A separate crop was not required because all primary typography, icons, borders, controls, and content rows remain readable in these native screenshots.

## Required Fidelity Surfaces

- Fonts and typography: project sans-serif stack is preserved; the hero title uses stable breakpoint sizes rather than viewport-scaled type, explicit zero letter spacing, per-character transform-only entrance motion, deliberate Chinese line breaks, and no clipped display text.
- Spacing and layout rhythm: the first viewport leaves the product preview visible, sticky scenes fill the usable viewport below the 64px header, and later sections use open bands rather than nested card grids.
- Colors and visual tokens: existing `surface`, `primary`, `secondary`, emerald, and amber tokens are preserved in both themes. The fantasy reference palette is intentionally not copied.
- Image quality and asset fidelity: no reference image is reused because the user requested motion-only reference. The primary visual is a code-native preview of the actual product workflow with the repository's existing Lucide icon family; no placeholder image or custom SVG substitute is present.
- Copy and content: above-the-fold copy is limited to the project brand, product value statement, one description, and two functional CTAs. No reference-site claims, navigation, metrics, or fantasy copy were introduced.
- Icons: all controls and status marks use the existing Lucide dependency with consistent stroke style and optical sizing.
- States and interactions: start/login, register, modal close, demo jump, three stage selectors, student/teacher tabs, theme toggle, smooth scrolling, reduced-motion static rendering, and mobile/desktop layouts were exercised.
- Accessibility: semantic headings, labelled stage navigation, 40px-or-larger controls, visible focus rings, `aria-current`, labelled product previews, and `prefers-reduced-motion` fallback are present.

## Findings

No actionable P0, P1, or P2 findings remain.

## Patches Made During QA

- Replaced the welcome layout's horizontal overflow behavior with scoped `overflow: clip` so the sticky story remains attached to the viewport.
- Corrected stage jump coordinates to include the sticky header's document offset.
- Hid inactive story layers after each crossfade to remove compositor ghosting and reduce paint work.
- Lowered the initial product window to prevent CTA overlap on desktop and mobile.
- Added a deliberate two-line learning-loop heading to remove a desktop orphan line.
- Shortened mobile stage labels and hid desktop-only analytics sidebars on small screens.
- Added a reduced-motion-safe title entrance and disabled the title sheen when motion reduction is requested.
- Added student and teacher views with realistic project functions sourced from the existing guide, student, and teacher screens.
- Added a three-stage learning timeline that collapses to a vertical rail on mobile instead of becoming a card stack.
- Lowered the initial product preview after increasing the title scale, leaving a measured 7px desktop and 16px mobile gap below the secondary CTA.

## Residual Risk

- Safari and Firefox were not separately exercised; the implementation uses standard sticky positioning, transforms, opacity, and `overflow: clip` supported by the project's modern browser baseline.
- The isolated frontend server has no backend on port 8000, so application startup records the existing token-refresh failure in development logs. The public page, modal, and all welcome interactions continue to render correctly.

## Verification

- `npm test`: 54 files and 345 tests passed.
- `npm run lint`: passed with no lint errors.
- `npm run build`: passed; existing Browserslist age and large vendor-chunk advisories remain unchanged.
- In-app Browser: page identity, meaningful DOM, no framework overlay, role-tab interaction, light/dark themes, and 1280 x 720 plus 390 x 844 layouts passed.
- Layout measurements: no horizontal overflow; the final CTA-to-preview gap is positive on both checked viewports.

## Above-The-Fold Copy Diff

Allowed visible copy is present and ordered as designed: `高数智学`, `真正理解一道题，然后走向下一步。`, the product description, `开始学习`, and `查看产品演示`. No unapproved eyebrow, badge, metric, or reference-site copy appears.

## Implementation Checklist

- [x] First viewport and next-scene preview verified.
- [x] Three scroll stages and selector jumps verified.
- [x] Desktop and mobile responsive states verified.
- [x] Light and dark themes verified.
- [x] Login modal open and close verified.
- [x] Student and teacher role views switch content and selected state.
- [x] Pre-class, active-learning, and review stages verified on desktop and mobile.
- [x] No horizontal overflow at 1280 x 720 or 390 x 844.
- [x] Reduced-motion path covered by automated test.
- [x] Source and implementation opened together in a combined comparison board.

final result: passed
