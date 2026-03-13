# tuiwall (v0.1.0)

[![Go Report Card](https://goreportcard.com/badge/github.com/Mug-Costanza/tuiwall)](https://goreportcard.com/report/github.com/Mug-Costanza/tuiwall)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/Mug-Costanza/tuiwall)](https://github.com/Mug-Costanza/tuiwall/releases)

## EASY-INSTALL TERMINAL HEADER WALLPAPERS

<table style="width: 100%; border-collapse: collapse;">
  <tr>
    <td style="width: 50%; border: none;">
      <img src="https://raw.githubusercontent.com/Mug-Costanza/tuiwall-presets/main/images/matrix.gif" alt="Matrix" width="100%">
    </td>
    <td style="width: 50%; border: none;">
      <img src="https://raw.githubusercontent.com/Mug-Costanza/tuiwall-presets/main/images/donut.gif" alt="Donut" width="100%">
    </td>
  </tr>
  <tr>
    <td style="width: 50%; border: none;">
      <img src="https://raw.githubusercontent.com/Mug-Costanza/tuiwall-presets/main/images/rain.gif" alt="Rain" width="100%">
    </td>
    <td style="width: 50%; border: none;">
      <img src="https://raw.githubusercontent.com/Mug-Costanza/tuiwall-presets/main/images/clock.gif" alt="Clock" width="100%">
    </td>
  </tr>
</table>

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
```bash
tuiwall enable
```
```bash
tuiwall disable
```

**Quick reset (disable & enable):**
```bash
tuiwall reset
```

**Change and List presets:**
```bash
tuiwall set <preset-name>
```
```bash
tuiwall list
```
```bash
tuiwall search <keyword>
```

**Check current status:**
```bash
tuiwall status
```

---

### INSTALLING / MANAGING PRESETS

**Install presets from the community repo or a specific URL:**
```bash
tuiwall install <preset-name>
```
```bash
tuiwall install <repo-url>
```

**Uninstall a preset:**
```bash
tuiwall uninstall <preset-name>
```

---

### CREATING / CONTRIBUTING PRESETS

**Create, edit, or locate a preset:**
```bash
tuiwall preset new <name>
```
```bash
tuiwall preset edit <name>
```
```bash
tuiwall preset path <name>
```

**Display preset metadata and attach images:**
```bash
tuiwall preset info <preset-name>
```
```bash
tuiwall preset image <preset-name> <image-path>
```

**Record a demo GIF and share:**
```bash
tuiwall record <preset-name>
```
```bash
tuiwall upload <preset-name>
```

---

### WARNING
**Vet every preset yourself via the editor prior to running, as each preset is a Python script.** Recommended presets can be downloaded from the community repo at: https://github.com/Mug-Costanza/tuiwall-presets
