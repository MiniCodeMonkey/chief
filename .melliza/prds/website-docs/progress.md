## Codebase Patterns
- Global Vue components registered via `app.component()` in `enhanceApp` can be used directly in markdown files without imports
- VitePress docs live in `/docs` directory with `.vitepress/config.ts` for configuration
- Use `npm run docs:dev` to start dev server, `npm run docs:build` for production build
- Base URL is `/melliza/` for GitHub Pages project site
- Custom theme lives in `docs/.vitepress/theme/` with `index.ts` extending DefaultTheme
- Vue components for theme go in `docs/.vitepress/theme/components/` directory
- Custom layouts extend DefaultTheme.Layout and can use slots like `#home-hero-before`
- Links in Vue components must include base path (e.g., `/melliza/guide/` not `/guide/`)
- Tailwind CSS v4 uses `@import "tailwindcss"` directive (not v3's `@tailwind` directives)
- Tailwind v4 plugin is configured via VitePress's `vite.plugins` option in config.ts
- Tokyo Night color palette defined in `tailwind.css` with both Tailwind v4 `@theme` and VitePress CSS variables
- Site is forced dark mode via `appearance: 'force-dark'` in config.ts
- Code blocks use Shiki's `tokyo-night` theme via `markdown.theme` in config.ts
- Links in markdown files should NOT include base path (use `/guide/quick-start` not `/melliza/guide/quick-start`)
- Links in Vue components MUST include base path (use `/melliza/guide/quick-start` not `/guide/quick-start`)
- Use `transformPageData` in config.ts to inject build-time data into pages via `pageData.frontmatter.head` script tags
- The `#doc-before` slot in DefaultTheme.Layout only renders on doc pages, not on home/landing pages

---

## 2026-01-28 - US-001
- **What was implemented**: VitePress project scaffolded in /docs directory
- **Files changed**:
  - `.gitignore` - added node_modules and VitePress build artifacts
  - `docs/package.json` - created with dev/build/preview scripts
  - `docs/package-lock.json` - dependency lockfile
  - `docs/.vitepress/config.ts` - site configuration with title "Melliza", tagline "Autonomous PRD Agent", base URL `/melliza/`
  - `docs/index.md` - landing page with hero layout
  - `docs/guide/index.md` - placeholder getting started page
- **Learnings for future iterations:**
  - VitePress v1.6.x uses Vite under the hood, configuration is in `.vitepress/config.ts`
  - The `base` option in config must match GitHub Pages project path (e.g., `/melliza/`)
  - VitePress creates `.vitepress/cache/` and `.vitepress/dist/` directories that should be gitignored
  - Dev server runs on port 5173 by default
---

## 2026-01-28 - US-002
- **What was implemented**: Tailwind CSS v4 integration with VitePress
- **Files changed**:
  - `docs/package.json` - added tailwindcss and @tailwindcss/vite dependencies
  - `docs/package-lock.json` - updated lockfile with new dependencies
  - `docs/.vitepress/config.ts` - added Tailwind v4 Vite plugin via `vite.plugins` option
  - `docs/.vitepress/theme/index.ts` - custom theme extending DefaultTheme and importing tailwind.css
  - `docs/.vitepress/theme/tailwind.css` - CSS file with `@import "tailwindcss"` directive
- **Learnings for future iterations:**
  - Tailwind CSS v4 uses `@import "tailwindcss"` directive instead of v3's `@tailwind base/components/utilities`
  - VitePress custom themes go in `.vitepress/theme/` with `index.ts` as entry point
  - The theme must re-export DefaultTheme to preserve VitePress default styling
  - Vite plugins are added to VitePress via the `vite` config option in `.vitepress/config.ts`
  - Tailwind v4 is purely CSS-based with no separate config file needed for basic setup
---

## 2026-01-28 - US-003
- **What was implemented**: Tokyo Night dark theme for the documentation site
- **Files changed**:
  - `docs/.vitepress/config.ts` - added `appearance: 'force-dark'` and `markdown.theme: 'tokyo-night'` for code blocks
  - `docs/.vitepress/theme/tailwind.css` - extensive Tokyo Night color palette and VitePress CSS variable overrides
- **Learnings for future iterations:**
  - VitePress uses `appearance: 'force-dark'` to force dark mode and hide the theme toggle
  - Shiki (VitePress's syntax highlighter) has built-in `tokyo-night` theme - just set `markdown.theme: 'tokyo-night'`
  - VitePress CSS variables are organized by category: `--vp-c-brand-*`, `--vp-c-bg-*`, `--vp-c-text-*`, etc.
  - Tailwind v4 uses `@theme` directive to define custom color utilities (e.g., `--color-tokyo-bg` becomes `bg-tokyo-bg`)
  - VitePress class names like `.VPSidebar`, `.VPNav`, `.VPContent` can be styled directly
  - Custom block (tip, warning, danger) colors use `--vp-c-tip-*`, `--vp-c-warning-*`, `--vp-c-danger-*` variables
  - Force dark mode for non-.dark html with duplicate CSS variables to prevent flash of light theme
---

## 2026-01-28 - US-004
- **What was implemented**: Landing page hero section with animated terminal
- **Files changed**:
  - `docs/.vitepress/theme/components/Hero.vue` - new custom Hero component with headline, terminal animation, install command, and CTA buttons
  - `docs/.vitepress/theme/HomeLayout.vue` - custom layout that hides default VitePress hero and uses custom Hero component
  - `docs/.vitepress/theme/index.ts` - updated to use HomeLayout as the main layout
- **Learnings for future iterations:**
  - VitePress allows custom layouts via named slots like `#home-hero-before`, `#home-hero-info`, etc.
  - To completely replace the default hero, use a custom Layout component that extends DefaultTheme.Layout
  - Hide default VitePress hero with `.VPHome .VPHero { display: none !important; }`
  - CSS animations with `animation-delay` can create sequenced typing/fadeIn effects for terminal output
  - Vue components in VitePress theme go in `.vitepress/theme/components/` directory
  - Links in Vue components should use the full base path (e.g., `/melliza/guide/` not `/guide/`)
---

## 2026-01-28 - US-005
- **What was implemented**: Landing page "How It Works" section with three-step visual workflow
- **Files changed**:
  - `docs/.vitepress/theme/components/HowItWorks.vue` - new component with three steps: Write PRD → Melliza Runs Loop → Code Gets Built
  - `docs/.vitepress/theme/HomeLayout.vue` - updated to include HowItWorks component after Hero
- **Learnings for future iterations:**
  - Landing page sections are added to HomeLayout via the `#home-hero-before` slot after other components
  - Tokyo Night color variables can be used directly in component styles (e.g., `#7aa2f7` for accent, `#bb9af7` for purple, `#9ece6a` for green)
  - SVG icons from Feather Icons work well for step illustrations
  - Flexbox with `flex-direction: column` on mobile and row on desktop handles responsive step layouts
  - Step connectors (arrows) should rotate 90 degrees on mobile to maintain visual flow
---

## 2026-01-28 - US-006
- **What was implemented**: Landing page "Key Features" section with four feature cards in a grid layout
- **Files changed**:
  - `docs/.vitepress/theme/components/Features.vue` - new component with 4 feature cards: Single Binary, Self-Contained State, Works Anywhere, Beautiful TUI
  - `docs/.vitepress/theme/HomeLayout.vue` - updated to include Features component after HowItWorks
- **Learnings for future iterations:**
  - CSS Grid with `grid-template-columns: repeat(2, 1fr)` creates a 2-column layout that gracefully collapses to 1 column on mobile
  - Different hover border colors for each card can be achieved with `:nth-child(n)` selectors
  - Use `rgba()` for semi-transparent background colors on feature icons (e.g., `rgba(122, 162, 247, 0.1)`)
  - Alternate section backgrounds between `#1a1b26` and `#16161e` for visual separation
  - The cyan color for Tokyo Night is `#7dcfff` (useful for UI/TUI related icons)
---

## 2026-01-28 - US-007
- **What was implemented**: Landing page footer with CTA section
- **Files changed**:
  - `docs/.vitepress/theme/components/Footer.vue` - new footer component with "Ready to automate your PRDs?" CTA, links to quick start guide and GitHub, and copyright notice
  - `docs/.vitepress/theme/HomeLayout.vue` - updated to include Footer component after Features
- **Learnings for future iterations:**
  - Footer sections can be placed in the `#home-hero-before` slot along with other landing page sections
  - Use `border-top` to visually separate footer from content above
  - CTA buttons follow same styling pattern as Hero: primary (filled) and secondary (outlined)
  - Dynamic year in copyright: `{{ new Date().getFullYear() }}` works in Vue template
  - Footer uses darker `#16161e` background to contrast with `#1a1b26` features section above
---

## 2026-01-28 - US-008
- **What was implemented**: Navigation and sidebar structure with all documentation pages
- **Files changed**:
  - `docs/.vitepress/config.ts` - added top nav (Home, Docs, GitHub link) and full sidebar configuration
  - `docs/guide/index.md` - updated getting started landing page with links to subpages
  - `docs/guide/quick-start.md` - new quick start guide
  - `docs/guide/installation.md` - new detailed installation guide
  - `docs/concepts/how-it-works.md` - new overview of how Melliza works
  - `docs/concepts/ralph-loop.md` - new deep dive into the Ralph Loop
  - `docs/concepts/prd-format.md` - new PRD format documentation
  - `docs/concepts/melliza-directory.md` - new .melliza directory guide
  - `docs/reference/cli.md` - new CLI reference
  - `docs/reference/configuration.md` - new configuration docs
  - `docs/reference/prd-schema.md` - new PRD schema reference
  - `docs/troubleshooting/common-issues.md` - new common issues guide
  - `docs/troubleshooting/faq.md` - new FAQ page
- **Learnings for future iterations:**
  - VitePress sidebar is configured via `themeConfig.sidebar` array with `text` and `items` for sections
  - Links in markdown files should NOT include base path (VitePress handles it automatically)
  - VitePress validates dead links during build - useful for catching broken internal links
  - Navigation items can be simple links or have children for dropdowns
  - Mobile navigation automatically collapses to hamburger menu (built into VitePress)
---

## 2026-01-28 - US-009
- **What was implemented**: Comprehensive Quick Start guide with all installation options and step-by-step instructions
- **Files changed**:
  - `docs/guide/quick-start.md` - expanded from placeholder to full guide with prerequisites, installation options (Homebrew, install script, from source), step-by-step workflow, TUI explanation with keyboard controls, and next steps links
- **Learnings for future iterations:**
  - VitePress `::: code-group` syntax creates tabbed code blocks for showing multiple installation options
  - Tables in markdown work well for keyboard shortcuts reference (pipe-separated columns with header row)
  - VitePress custom blocks `::: tip`, `::: info`, `::: warning` are useful for highlighting prerequisites and notes
  - Quick start guides should focus on "get running fast" with links to deeper docs, not exhaustive detail
---

## 2026-01-28 - US-010
- **What was implemented**: Detailed installation guide with all platform coverage
- **Files changed**:
  - `docs/guide/installation.md` - expanded from basic guide to comprehensive installation reference with prerequisites at top, Homebrew with update instructions, install script options table, complete platform matrix, platform-specific code tabs for manual download, building from source with version embedding, and thorough verification section
- **Learnings for future iterations:**
  - `::: code-group` is excellent for platform-specific installation commands (one tab per platform)
  - Architecture detection commands: `uname -m` returns `arm64`/`x86_64` on macOS, `aarch64`/`x86_64` on Linux
  - VitePress `::: info` blocks are good for notes about PATH configuration
  - `::: warning` blocks work well for troubleshooting tips at the end of installation sections
  - Tables are good for option flags documentation (Option | Description | Example format)
---

## 2026-01-28 - US-012
- **What was implemented**: Comprehensive Ralph Loop deep dive page with detailed step-by-step explanation
- **Files changed**:
  - `docs/concepts/ralph-loop.md` - expanded from basic overview to full deep dive with:
    - Updated blog post link to actual URL (larswadefalk.com)
    - Enhanced Mermaid flowchart with story selection, iteration limits, and Tokyo Night color styling
    - 7 detailed steps (Read State, Select Next Story, Build Prompt, Invoke Gemini Code, Stream & Parse Output, Watch for Completion Signal, Update and Continue)
    - Tables showing files read and what Melliza learns from each
    - Story selection logic explanation (priority sorting, inProgress handling)
    - Simplified example of the embedded prompt Gemini receives
    - ASCII diagram showing stream-json output format with message types
    - Detailed explanation of `<melliza-complete/>` signal and what it implies
    - Iteration limits section with scenario table and troubleshooting tips
    - "What's Next" links to related docs
- **Learnings for future iterations:**
  - Mermaid flowcharts support `style` directives for Tokyo Night colors (fill, stroke, color)
  - Use `([text])` for stadium-shaped (rounded) nodes in Mermaid for start/end states
  - Tables are effective for showing file-to-purpose mappings
  - Code blocks with ASCII box drawing characters create effective stream visualizations
  - Numbered lists with bold step names and sub-bullets create scannable deep dive content
---

## 2026-01-28 - US-011
- **What was implemented**: Enhanced "How Melliza Works" overview page with comprehensive documentation
- **Files changed**:
  - `docs/concepts/how-it-works.md` - expanded from basic placeholder to full overview with:
    - High-level explanation of autonomous agent concept vs traditional interactive prompting
    - Improved ASCII diagram showing: User → PRD → Melliza → Gemini → Code pipeline
    - Component table explaining each part of the system
    - Detailed 7-step iteration flow explaining how stories are processed
    - New "Conventional Commits" section showing commit message format
    - New "Progress Tracking" section explaining progress.md and learnings
    - Link to blog post in a tip callout at the top
    - Updated links to related docs using em-dash formatting
- **Learnings for future iterations:**
  - VitePress `::: tip` blocks with custom headers work well for important links/callouts
  - ASCII art diagrams should fit within 80 characters for readability
  - Tables are effective for explaining system components with Role descriptions
  - Numbered lists with bold step names create scannable process documentation
  - Using em-dashes (—) for link descriptions creates consistent visual style
---

## 2026-01-28 - US-013
- **What was implemented**: Comprehensive .melliza directory guide with detailed structure and file explanations
- **Files changed**:
  - `docs/concepts/melliza-directory.md` - expanded from basic placeholder to full guide with:
    - Enhanced directory tree showing project context (not just `.melliza/` in isolation)
    - Detailed `prds/` subdirectory explanation with CLI usage
    - Expanded file explanations with field tables for `prd.json`, example entries for `progress.md`, and usage context for each file
    - "Self-Contained by Design" section emphasizing no global config, no conflicts, no cleanup
    - Enhanced portability section with multiple examples (move, clone, remote)
    - Multiple PRDs section with practical examples
    - Git considerations with tables for commit/ignore decisions and `.gitignore` pattern
    - "What's Next" navigation links
- **Learnings for future iterations:**
  - VitePress doesn't have built-in `gitignore` language for syntax highlighting — it falls back to `txt` (cosmetic warning only, not a build error)
  - Tables with Yes/No columns and "Why" explanations are effective for commit/ignore decisions
  - Directory trees that show surrounding project context (e.g., `src/`, `package.json`) help users understand where `.melliza/` fits
  - `::: tip` blocks work well for collaborative workflow notes
---

## 2026-01-28 - US-015
- **What was implemented**: Comprehensive CLI Reference page with complete command documentation
- **Files changed**:
  - `docs/reference/cli.md` - expanded from basic reference to comprehensive CLI documentation with:
    - Top-level usage overview with command summary table (mirrors `melliza --help` structure)
    - Enhanced `melliza` (default) command with all flags, defaults, and multiple examples including combined flags
    - Enhanced `melliza new` with interactive prompt details, directory structure output, and example walkthrough
    - Enhanced `melliza edit` with `$EDITOR` tips and flag documentation
    - Enhanced `melliza status` with example output showing completion counts
    - Enhanced `melliza list` with example output showing multi-PRD overview
    - Improved TUI keyboard shortcuts section with panel description tip
    - Exit codes section with scripting example using `$?`
    - Environment variables table with equivalent flag column and override behavior examples
    - Section dividers and tip/info callouts throughout
- **Learnings for future iterations:**
  - Adding example output (commented-out style `# Example output:`) helps users understand what to expect without being misleading
  - Environment variable tables benefit from an "Equivalent Flag" column showing the CLI override
  - VitePress `::: tip` blocks are useful for editor configuration hints
  - Section dividers (`---`) between commands improve scanability in long reference docs
---

## 2026-01-28 - US-014
- **What was implemented**: Comprehensive PRD Format Reference page with full documentation of both `prd.md` and `prd.json` formats
- **Files changed**:
  - `docs/concepts/prd-format.md` - expanded from basic placeholder to comprehensive reference with:
    - Detailed `prd.md` guidance with what to include and a full example
    - Complete `prd.json` schema documentation with top-level, settings, and UserStory field tables
    - Story selection logic explanation with priority walkthrough table and `inProgress` behavior
    - Fully annotated example PRD with comments explaining every field
    - Best practices section: specific acceptance criteria, keeping stories small, ordering by dependency, consistent IDs, giving Gemini context
    - "What's Next" navigation links
- **Learnings for future iterations:**
  - JSON doesn't support comments, so annotated examples need an `::: info` callout explaining annotations are for illustration only
  - Good/bad comparison code blocks are effective for showing best practices (use `// ✓ Good` and `// ✗ Bad` prefixes)
  - Tables with Story/Priority/Passes/Selected columns are effective for explaining selection logic
  - The PRD schema reference page (`/reference/prd-schema`) already has TypeScript interfaces — the concepts page should focus on practical usage, not duplicate type definitions
---

## 2026-01-28 - US-016
- **What was implemented**: Verified existing Troubleshooting Guide meets all acceptance criteria
- **Files changed**:
  - `docs/troubleshooting/common-issues.md` - already had comprehensive content from US-008 covering all 6 required issues (Gemini not found, Permission denied, No sound on completion, PRD not updating, Loop not progressing, Max iterations reached) plus 2 additional issues (No PRD Found, Invalid JSON)
  - `.melliza/prds/website-docs/prd.json` - marked US-016 as passes: true
- **Learnings for future iterations:**
  - Some stories may already be implemented by earlier stories that created placeholder pages with full content (US-008 created all doc pages with initial content)
  - Always verify the build passes even when no new content is added — the build check is the quality gate
  - The troubleshooting page uses a consistent Symptom/Cause/Solution pattern with code examples, which is a good format for debugging guides
---

## 2026-01-28 - US-017
- **What was implemented**: LLM Actions Vue component with dropdown menu for copying page markdown and opening in ChatGPT/Gemini
- **Files changed**:
  - `docs/.vitepress/theme/components/LlmActions.vue` - new component with dropdown menu containing "Copy as Markdown", "Open in ChatGPT", and "Open in Gemini" actions
  - `docs/.vitepress/config.ts` - added `transformPageData` hook to inject raw markdown content into `window.__DOC_RAW` via head script tag
  - `docs/.vitepress/theme/HomeLayout.vue` - integrated LlmActions component into `#doc-before` slot so it appears on doc pages only (not landing page)
- **Learnings for future iterations:**
  - VitePress `transformPageData` hook runs at build time and can inject scripts via `pageData.frontmatter.head` push
  - Use `siteConfig.srcDir` (not `siteConfig.root`) to construct file paths for reading markdown source in transformers
  - The `#doc-before` slot in DefaultTheme.Layout renders content above doc pages only — it does not appear on home layout pages
  - `JSON.stringify()` is essential for safely embedding raw markdown in a `<script>` tag (handles newlines, quotes, etc.)
  - Click-outside pattern for dropdowns: add `document.addEventListener('click', handler)` in `onMounted` and clean up in `onUnmounted`
  - ChatGPT URL format: `https://chatgpt.com/?q=<encoded>`, Gemini URL format: `https://gemini.ai/new?q=<encoded>`
---

## 2026-01-28 - US-018
- **What was implemented**: VitePress local search enabled via built-in search provider
- **Files changed**:
  - `docs/.vitepress/config.ts` - added `search: { provider: 'local' }` to `themeConfig`
  - `.melliza/prds/website-docs/prd.json` - marked US-018 as passes: true
- **Learnings for future iterations:**
  - VitePress built-in local search (`provider: 'local'`) covers all common search needs: `/` key trigger, search icon in nav bar, page titles and content previews, and keyboard navigation — no additional configuration needed
  - The search configuration goes in `themeConfig.search`, not at the top-level config
  - No additional packages needed — local search is built into VitePress core
---

## 2026-01-28 - US-019
- **What was implemented**: Screenshot and recording placeholder components with placeholders on doc pages and images directory
- **Files changed**:
  - `docs/.vitepress/theme/components/PlaceholderImage.vue` - new reusable placeholder component with customizable dimensions and label, styled with Tokyo Night colors and dashed border
  - `docs/.vitepress/theme/components/AsciinemaPlaceholder.vue` - new placeholder for terminal recordings with embed instructions
  - `docs/.vitepress/theme/index.ts` - registered PlaceholderImage and AsciinemaPlaceholder as global components via `enhanceApp`
  - `docs/guide/quick-start.md` - added placeholders for melliza new flow, TUI dashboard, and asciinema recording
  - `docs/reference/cli.md` - added TUI log view placeholder in keyboard shortcuts section
  - `docs/public/images/README.md` - created with list of required screenshots, recording specs, and image guidelines
  - `.melliza/prds/website-docs/prd.json` - marked US-019 as passes: true
- **Learnings for future iterations:**
  - VitePress global components are registered in `enhanceApp({ app })` in the theme's `index.ts` using `app.component()`
  - Global components can be used directly in markdown files without imports (e.g., `<PlaceholderImage label="..." />`)
  - The `docs/public/` directory serves static assets at the site root (e.g., `docs/public/images/foo.png` → `/melliza/images/foo.png`)
  - Vue component props with default values use `defineProps<{}>()` with optional types (e.g., `width?: string`)
---

## 2026-01-28 - US-020
- **What was implemented**: GitHub Actions workflow for automatic deployment to GitHub Pages
- **Files changed**:
  - `.github/workflows/docs.yml` - new workflow file that builds VitePress site and deploys to GitHub Pages on push to main
  - `.melliza/prds/website-docs/prd.json` - marked US-020 as passes: true
- **Learnings for future iterations:**
  - GitHub Pages deployment uses a two-job pipeline: `build` (upload artifact) and `deploy` (deploy pages)
  - `actions/configure-pages@v5`, `actions/upload-pages-artifact@v3`, and `actions/deploy-pages@v4` are the current recommended actions
  - The workflow needs `permissions: pages: write, id-token: write, contents: read` for Pages deployment
  - `concurrency` group with `cancel-in-progress: false` prevents concurrent deployments from conflicting
  - `workflow_dispatch` trigger allows manual deployment without a code push
  - The `paths` filter on push trigger limits builds to changes in `docs/` or the workflow file itself
  - npm ci with `cache-dependency-path` pointing to `docs/package-lock.json` enables dependency caching
---

## 2026-01-28 - US-021
- **What was implemented**: SEO and social cards with meta descriptions, Open Graph tags, Twitter cards, favicon, and social image
- **Files changed**:
  - `docs/.vitepress/config.ts` - added `head` array with favicon, OG tags, and Twitter card tags
  - `docs/public/favicon.ico` - new favicon with "C" on Tokyo Night background
  - `docs/public/images/og-default.png` - new 1200x630 OG social image with Tokyo Night styling
  - All documentation pages - added frontmatter `description` field for per-page meta descriptions:
    - `docs/index.md`, `docs/guide/index.md`, `docs/guide/quick-start.md`, `docs/guide/installation.md`
    - `docs/concepts/how-it-works.md`, `docs/concepts/ralph-loop.md`, `docs/concepts/prd-format.md`, `docs/concepts/melliza-directory.md`
    - `docs/reference/cli.md`, `docs/reference/configuration.md`, `docs/reference/prd-schema.md`
    - `docs/troubleshooting/common-issues.md`, `docs/troubleshooting/faq.md`
  - `.melliza/prds/website-docs/prd.json` - marked US-021 as passes: true
- **Learnings for future iterations:**
  - VitePress auto-generates `<meta name="description">` from the page's frontmatter `description` or site-level `description` config
  - Don't put explicit `<meta name="description">` in the `head` array — it will override per-page frontmatter descriptions
  - ImageMagick can create simple OG images: `magick -size 1200x630 xc:'#color' -font "Helvetica-Bold" -pointsize 72 -fill '#color' -gravity center -annotate +0+0 'Text' output.png`
  - Favicon can be created with ImageMagick: `magick -size 128x128 xc:'#bg' ... -resize 32x32 favicon.ico`
  - OG image URL must be absolute (e.g., `https://minicodemonkey.github.io/melliza/images/og-default.png`)
  - VitePress frontmatter `description` appears as `<meta name="description">` in the rendered HTML
---

## 2026-01-28 - US-022
- **What was implemented**: Mobile responsiveness verification and improvements across the site
- **Files changed**:
  - `docs/.vitepress/theme/tailwind.css` - added comprehensive mobile CSS including:
    - Code blocks with horizontal scroll (`overflow-x: auto`, `-webkit-overflow-scrolling: touch`)
    - Tables with horizontal scroll wrapper
    - Touch targets minimum 44px height for interactive elements (sidebar links, nav items, search buttons)
    - Font size adjustments for code and tables at 768px and 420px breakpoints
    - Tighter spacing at 375px (420px) breakpoint
  - `docs/.vitepress/theme/components/Hero.vue` - added 375px (420px) breakpoint with:
    - Smaller hero headline and subheadline font sizes
    - Smaller terminal font size and padding
    - Touch target minimum height (44px) for buttons and copy button
  - `.melliza/prds/website-docs/prd.json` - marked US-022 as passes: true
- **Learnings for future iterations:**
  - VitePress navigation automatically collapses to hamburger menu on mobile — no custom implementation needed
  - Touch targets should be minimum 44px for accessibility (Apple/Google HIG recommendation)
  - Use `-webkit-overflow-scrolling: touch` for smooth momentum scrolling on iOS
  - Tables with `display: block; overflow-x: auto` allow horizontal scrolling while preserving table structure
  - Landing page components (Hero, HowItWorks, Features, Footer) already had 640px breakpoints from initial implementation — just needed to add 375px/420px for very small screens
  - VitePress uses 768px as the mobile breakpoint for sidebar collapse
---
