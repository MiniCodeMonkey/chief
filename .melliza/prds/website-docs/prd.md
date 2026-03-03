# Melliza Documentation Website

## Overview

A VitePress documentation site for Melliza, hosted on GitHub Pages at `minicodemonkey.github.io/melliza`. The site includes a stunning landing page and comprehensive documentation explaining how Melliza works, with a Tokyo Night-inspired dark theme and Tailwind CSS styling.

## Goals

- Provide a beautiful, modern landing page that explains what Melliza is and why developers need it
- Document the complete PRD → Ralph loop workflow with clear diagrams
- Offer a full CLI reference generated from the codebase
- Enable easy sharing of docs with LLMs (copy as markdown, open in Gemini/ChatGPT)
- Deploy automatically via GitHub Actions

## Key Messages

The documentation should emphasize Melliza's core philosophy:

1. **Single binary, zero dependencies** - Drop it on any machine and it works
2. **All state in `.melliza/`** - Portable, self-contained, nothing global
3. **Works anywhere** - Local dev machine or remote server, same experience
4. **Simple core** - The loop is ~80 lines, easy to understand and debug

## External Resources

- **Blog Post:** [Ship Features in Your Sleep with Ralph Loops](https://www.geocod.io/code-and-coordinates/2026-01-27-ralph-loops/) - Background on the motivation and concepts behind Ralph loops. Should be linked prominently in the "How Melliza Works" and "Ralph Loop" documentation pages.

## User Stories

### US-001: VitePress Project Setup
**Priority:** 1
**Description:** As a developer, I want a VitePress project scaffolded in `/docs` so that I can build the documentation site.

**Acceptance Criteria:**
- [ ] VitePress installed and configured in `/docs` directory
- [ ] `package.json` with dev/build/preview scripts
- [ ] Basic `.vitepress/config.ts` with site metadata
- [ ] Site title: "Melliza" with tagline "Autonomous PRD Agent"
- [ ] Base URL configured for `/melliza/` (GitHub Pages project site)
- [ ] `npm run docs:dev` starts the dev server successfully

### US-002: Tailwind CSS v4 Integration
**Priority:** 2
**Description:** As a developer, I want Tailwind CSS v4 integrated with VitePress so that I can style the landing page with utility classes.

**Acceptance Criteria:**
- [ ] Tailwind CSS v4 installed (`@tailwindcss/vite` plugin)
- [ ] Vite config updated to use Tailwind v4 plugin
- [ ] CSS file with `@import "tailwindcss"` directive
- [ ] Utility classes work in Vue components and markdown
- [ ] No conflicts with VitePress default styles

### US-003: Tokyo Night Theme Configuration
**Priority:** 3
**Description:** As a user, I want a dark theme inspired by Tokyo Night so that the site has a modern, developer-friendly aesthetic.

**Acceptance Criteria:**
- [ ] Custom CSS variables for Tokyo Night color palette
- [ ] Colors: background `#1a1b26`, foreground `#a9b1d6`, accent `#7aa2f7`, green `#9ece6a`, red `#f7768e`, yellow `#e0af68`, purple `#bb9af7`, cyan `#7dcfff`
- [ ] Tailwind v4 theme extension using `@theme` directive for custom colors
- [ ] VitePress theme overrides for sidebar, nav, and content
- [ ] Code blocks use Tokyo Night syntax highlighting
- [ ] Consistent dark mode throughout (no light mode toggle needed)

### US-004: Landing Page Hero Section
**Priority:** 4
**Description:** As a visitor, I want an impactful hero section so that I immediately understand what Melliza does.

**Acceptance Criteria:**
- [ ] Large headline: "Autonomous PRD Agent"
- [ ] Subheadline explaining the value proposition in one sentence
- [ ] Animated terminal showing `melliza new` → `melliza` workflow (CSS animation, not real recording)
- [ ] Install command with copy button: `brew install minicodemonkey/melliza/melliza`
- [ ] "Get Started" button linking to quick start docs
- [ ] "View on GitHub" secondary button
- [ ] Responsive design (looks good on mobile)

### US-005: Landing Page "How It Works" Section
**Priority:** 5
**Description:** As a visitor, I want a visual explanation of Melliza's workflow so that I understand the concept before diving into docs.

**Acceptance Criteria:**
- [ ] Three-step visual flow: Write PRD → Melliza Runs Loop → Code Gets Built
- [ ] Simple icons or illustrations for each step
- [ ] Brief description under each step (1-2 sentences)
- [ ] Emphasize autonomous nature - "you watch, Gemini works"

### US-006: Landing Page Key Features Section
**Priority:** 6
**Description:** As a visitor, I want to see Melliza's key features highlighted so that I understand why it's valuable.

**Acceptance Criteria:**
- [ ] Feature: "Single Binary" - No runtime dependencies, download and run
- [ ] Feature: "Self-Contained State" - Everything in `.melliza/`, fully portable
- [ ] Feature: "Works Anywhere" - Local machine or remote server, SSH in and run
- [ ] Feature: "Beautiful TUI" - Real-time progress, keyboard controls
- [ ] Each feature has icon, title, and 1-2 sentence description
- [ ] Grid layout, responsive

### US-007: Landing Page Footer with CTA
**Priority:** 7
**Description:** As a visitor, I want a footer section so that I have clear next steps.

**Acceptance Criteria:**
- [ ] Final CTA: "Ready to automate your PRDs?"
- [ ] Link to quick start guide
- [ ] Link to GitHub repository
- [ ] Copyright notice

### US-008: Navigation and Sidebar Structure
**Priority:** 8
**Description:** As a user, I want clear navigation so that I can find documentation easily.

**Acceptance Criteria:**
- [ ] Top nav: Home, Docs, GitHub link
- [ ] Sidebar sections: Getting Started, Concepts, Reference, Troubleshooting
- [ ] Getting Started: Quick Start, Installation
- [ ] Concepts: How Melliza Works, The Ralph Loop, PRD Format, The .melliza Directory
- [ ] Reference: CLI Commands, Configuration, PRD Schema
- [ ] Troubleshooting: Common Issues, FAQ
- [ ] Mobile-friendly navigation

### US-009: Quick Start Guide
**Priority:** 9
**Description:** As a new user, I want a quick start guide so that I can get Melliza running in under 5 minutes.

**Acceptance Criteria:**
- [ ] Prerequisites section (Gemini Code CLI installed)
- [ ] Installation options (Homebrew, install script, from source)
- [ ] Step 1: Install Melliza (with copy-able commands)
- [ ] Step 2: Create your first PRD (`melliza new`)
- [ ] Step 3: Run the loop (`melliza`)
- [ ] Step 4: Watch it work (brief TUI explanation)
- [ ] "Next steps" linking to deeper docs

### US-010: Installation Guide
**Priority:** 10
**Description:** As a user, I want detailed installation instructions for all platforms so that I can install Melliza on my system.

**Acceptance Criteria:**
- [ ] Homebrew installation (macOS/Linux)
- [ ] Install script with options (version, custom dir)
- [ ] Manual binary download with platform matrix
- [ ] Building from source instructions
- [ ] Verifying installation (`melliza --version`)
- [ ] Prerequisites clearly listed

### US-011: How Melliza Works Overview
**Priority:** 11
**Description:** As a user, I want an overview of how Melliza works so that I understand the system before using it.

**Acceptance Criteria:**
- [ ] High-level explanation of the autonomous agent concept
- [ ] Diagram showing: User → PRD → Melliza → Gemini → Code
- [ ] Explanation of "one iteration = one story"
- [ ] Mention of conventional commits, progress tracking
- [ ] Link to the blog post for motivation and background
- [ ] Link to detailed Ralph Loop explanation

### US-012: The Ralph Loop Deep Dive
**Priority:** 12
**Description:** As a user, I want an in-depth explanation of the Ralph Loop so that I understand exactly what happens during execution.

**Acceptance Criteria:**
- [ ] Link to blog post "Ship Features in Your Sleep with Ralph Loops" at top for additional context
- [ ] Mermaid flowchart showing the loop: Read State → Build Prompt → Invoke Gemini → Stream Output → Check Completion → Repeat
- [ ] Explanation of each step with what files are read/written
- [ ] Diagram showing stream-json output parsing
- [ ] Explanation of how Gemini knows what to do (embedded prompt)
- [ ] Explanation of `<melliza-complete/>` signal
- [ ] Iteration limits and why they exist
- [ ] Simple, straightforward language - no jargon

### US-013: The .melliza Directory Guide
**Priority:** 13
**Description:** As a user, I want to understand the `.melliza/` directory structure so that I know where state is stored.

**Acceptance Criteria:**
- [ ] Directory tree visualization
- [ ] Explanation of `prds/` subdirectory structure
- [ ] File explanations: `prd.md`, `prd.json`, `progress.txt`, `gemini.log`
- [ ] Emphasis on portability - "move your project, state moves with it"
- [ ] Emphasis on self-contained nature - no global config, no home directory files
- [ ] Explanation of multiple PRDs in same project
- [ ] Git considerations (what to commit, what to ignore)

### US-014: PRD Format Reference
**Priority:** 14
**Description:** As a user, I want complete documentation of the PRD format so that I can write effective PRDs.

**Acceptance Criteria:**
- [ ] `prd.md` markdown format with examples
- [ ] `prd.json` schema documentation
- [ ] Field-by-field explanation with types and descriptions
- [ ] Story selection logic (priority, inProgress handling)
- [ ] Best practices for writing good user stories
- [ ] Example PRD with annotations

### US-015: CLI Reference
**Priority:** 15
**Description:** As a user, I want a complete CLI reference so that I know all available commands and options.

**Acceptance Criteria:**
- [ ] All commands documented: (default), init, edit, status, list
- [ ] All flags documented with descriptions and defaults
- [ ] Usage examples for each command
- [ ] Keyboard shortcuts reference (TUI controls)
- [ ] Exit codes documentation
- [ ] Structure mirrors `melliza --help` output

### US-016: Troubleshooting Guide
**Priority:** 16
**Description:** As a user, I want a troubleshooting guide so that I can solve common problems.

**Acceptance Criteria:**
- [ ] "Gemini not found" - installation verification
- [ ] "Permission denied" - explanation of `--dangerously-skip-permissions`
- [ ] "No sound on completion" - audio troubleshooting
- [ ] "PRD not updating" - file watcher issues
- [ ] "Loop not progressing" - checking gemini.log
- [ ] "Max iterations reached" - increasing limits
- [ ] Each issue has: symptom, cause, solution

### US-017: LLM Actions Component
**Priority:** 17
**Description:** As a user, I want to copy documentation pages as markdown or open them in Gemini/ChatGPT so that I can get AI help with Melliza.

**Acceptance Criteria:**
- [ ] Vue component with dropdown menu
- [ ] "Copy as Markdown" - copies raw page markdown to clipboard
- [ ] "Open in ChatGPT" - opens ChatGPT with prompt about the page
- [ ] "Open in Gemini" - opens Gemini with prompt about the page
- [ ] Component appears on every documentation page (not landing page)
- [ ] Styled to match Tokyo Night theme
- [ ] Raw markdown available via `__DOC_RAW` window variable (requires VitePress transformer config)

### US-018: Search Configuration
**Priority:** 18
**Description:** As a user, I want to search the documentation so that I can find information quickly.

**Acceptance Criteria:**
- [ ] VitePress local search enabled
- [ ] Search triggered by `/` or clicking search icon
- [ ] Results show page titles and content previews
- [ ] Keyboard navigation in search results

### US-019: Screenshot and Recording Placeholders
**Priority:** 19
**Description:** As a documentation author, I want placeholder images for screenshots and recordings so that the docs are ready for visual content.

**Acceptance Criteria:**
- [ ] Placeholder image component with customizable dimensions and label
- [ ] Placeholders for: TUI dashboard, TUI log view, melliza new flow
- [ ] Placeholder for asciinema recording embed (with instructions for later)
- [ ] `/docs/public/images/` directory created
- [ ] README in images folder explaining what screenshots are needed

### US-020: GitHub Actions Deployment
**Priority:** 20
**Description:** As a maintainer, I want automatic deployment to GitHub Pages so that docs update when I push to main.

**Acceptance Criteria:**
- [ ] `.github/workflows/docs.yml` workflow file
- [ ] Triggers on push to main (changes in /docs or workflow file)
- [ ] Builds VitePress site
- [ ] Deploys to GitHub Pages
- [ ] Uses `actions/deploy-pages` for deployment
- [ ] Workflow tested and working

### US-021: SEO and Social Cards
**Priority:** 21
**Description:** As a maintainer, I want proper SEO and social sharing cards so that the site looks good when shared.

**Acceptance Criteria:**
- [ ] Meta description for all pages
- [ ] Open Graph tags (title, description, image)
- [ ] Twitter card tags
- [ ] Favicon configured
- [ ] Default social image created (placeholder acceptable)

### US-022: Mobile Responsiveness Verification
**Priority:** 22
**Description:** As a mobile user, I want the site to work well on mobile devices so that I can read docs on my phone.

**Acceptance Criteria:**
- [ ] Landing page responsive at 375px, 768px, 1024px breakpoints
- [ ] Navigation collapses to hamburger menu on mobile
- [ ] Code blocks horizontally scroll on mobile
- [ ] Tables are readable on mobile (horizontal scroll or stacked)
- [ ] Touch targets are appropriately sized

## Non-Goals

- Light mode theme (dark only for v1)
- Blog or changelog section
- Internationalization (English only)
- Comments or feedback system
- Analytics integration
- API documentation (Melliza has no API)
- Video tutorials (text and diagrams only)

## Technical Considerations

- VitePress 1.x for static site generation
- Tailwind CSS v4 with `@tailwindcss/vite` plugin
- Vue 3 components for interactive elements
- Mermaid for diagrams (built into VitePress)
- GitHub Pages for hosting (project site at /melliza/)

## Success Metrics

- Site builds without errors
- All documentation pages render correctly
- Navigation works on desktop and mobile
- Search returns relevant results
- LLM actions component functions correctly
- GitHub Actions deploys successfully on merge to main
