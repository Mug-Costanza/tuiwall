# tuiwall (v0.1.0)

[![Go Report Card](https://goreportcard.com/badge/github.com/Mug-Costanza/tuiwall)](https://goreportcard.com/report/github.com/Mug-Costanza/tuiwall)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/Mug-Costanza/tuiwall)](https://github.com/Mug-Costanza/tuiwall/releases)

## EASY-INSTALL TERMINAL HEADER WALLPAPERS

https://github.com/Mug-Costanza/tuiwall-presets/blob/main/images/matrix.gif
https://github.com/Mug-Costanza/tuiwall-presets/blob/main/images/donut.gif
https://github.com/Mug-Costanza/tuiwall-presets/blob/main/images/rain.gif

### Presets
https://github.com/Mug-Costanza/tuiwall-presets

### What is this?
**tuiwall** creates persistent, animated, or informative headers at the top of your tmux windows. It allows you to customize the top ten lines of your terminal workspace with everything from CPU monitors and elaborate animated ASCII art to API-driven displays (live stock charts, weather reporting, etc.), all driven by simple Python scripts.

### Purpose
Improve the UX and customization options of terminal environments.

### Dependencies
* **Python 3** (Standard library only; no external packages required)
* **tmux**
* **git**
* **gh** (GitHub CLI for community features)
* **Text editor** (Respects $EDITOR; defaults to vi)
* **vhs** (Required only for the record feature)

---

### USAGE

**Enable / Disable header:**
\`\`\`bash
tuiwall enable
tuiwall disable
\`\`\`

**Quick reset (disable & enable):**
\`\`\`bash
tuiwall reset
\`\`\`

**Change and List presets:**
\`\`\`bash
tuiwall set <preset-name>
tuiwall list
tuiwall search <keyword>
\`\`\`

**Check current status:**
\`\`\`bash
tuiwall status
\`\`\`

---

### INSTALLING / MANAGING PRESETS

**Install presets from the community repo or a specific URL:**
\`\`\`bash
tuiwall install <preset-name>
tuiwall install <repo-url>
\`\`\`

**Uninstall a preset:**
\`\`\`bash
tuiwall uninstall <preset-name>
\`\`\`

---

### CREATING / CONTRIBUTING PRESETS

**Create, edit, or locate a preset:**
\`\`\`bash
tuiwall preset new <name>
tuiwall preset edit <name>
tuiwall preset path <name>
\`\`\`

**Display preset metadata and attach images:**
\`\`\`bash
tuiwall preset info <preset-name>
tuiwall preset image <preset-name> <image-path>
\`\`\`

**Record a demo GIF and share:**
\`\`\`bash
tuiwall record <preset-name>
tuiwall upload <preset-name>
\`\`\`

---

### WARNING
**Vet every preset yourself via the editor prior to running, as each preset is a Python script.** Recommended presets can be downloaded from the community repo at: https://github.com/Mug-Costanza/tuiwall-presets
