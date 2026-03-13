# TUIWALL (v0.1.0)

## EASY-INSTALL TERMINAL HEADER WALLPAPERS

### What is this?

	* Tuiwall creates persistent, animated, or informative headers at the top of your tmux windows. It allows you to customize the top ten lines of your terminal workspace with everything from CPU monitors, elaborate animated ASCII art, or even API-driven displays (live stock charts, weather reporting, etc.), all driven by simple Python scripts.

### Purpose
	
	Improve the UX and customization options of terminal environments 

### Dependencies

	* python (no external packages required)
	* tmux
	* git
	* gh 
	* Text editor (vi / vim / nvim, emacs, helix, nano) // Respects $EDITOR 
						            // (otherwise, default is vi)
	* vhs (for creating gifs)

### USAGE
	
	Enable / Disable header using
```bash
	tuiwall enable / disable
```

	Quick reset (disable & enable) using
```bash
	tuiwall reset
```			

	Change presets using
```bash
	tuiwall set clock / status
```

	List installed presets using
```bash
	tuiwall list
```

	Search through installed presets using
```bash
	tuiwall search <keyword>
```

	Status report using
```bash
	tuiwall status
```

### INSTALLING / MANAGING PRESETS

	Install presets from the community repo using
```bash
	tuiwall install	<preset>
```

	Install presets from the community repo using
```bash
	tuiwall install	<repo url>
```
	
	Uninstall presets using
```bash
	tuiwall uninstall <preset>
```

### CREATING / CONTRIBUTING PRESETS

	Edit / create presets using
```bash
	tuiwall preset <new | edit | path> <preset>
```

	Display additional preset info using
```bash
	tuiwall preset <info> <preset>
```

	Record a gif for your preset using
```bash
	tuiwall record preset
```

	Associate an image with a preset using
```bash
	tuiwall preset image <preset> <image path> 
```

	Copy a preexisting preset using
```bash
	tuiwall preset copy <preexisting preset> <preset>
```

	Upload presets to the community repo using
```bash
		uiwall upload <preset> <repo url | NULL>
```

	Upload presets to your own repo using
```bash
	tuiwall upload <preset> <repo url>
```

### WARNING : Vet every preset yourself via the editor prior to running since each preset is a Python script. Recommmended presets can be downloaded from the community repo at : https://github.com/Mug-Costanza/tuiwall-presets

