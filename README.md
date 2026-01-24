# TUIWALL (v0.1)

## EASY-INSTALL TERMINAL HEADER WALLPAPERS

### What is this?

	* Tuiwall is a pseudo wallpaper engine for the terminal
	* Essentially, tuiwall opens a 10-line tmux pane at the 
		top of each terminal / tmux window and runs any 
		custom Python curses script 

### Purpose
	
	Improve the UX and customization options of terminal environments 

### Dependencies

	* Python3 (no external packages required)
	* Tmux
	* Git
	* Text editor (vi / vim / nvim, emacs, helix, nano) // Respects $EDITOR 
						            // (otherwise, defualt is vi)

### USAGE
	
	Install using 
		brew install tuiwall // MacOS
		sudo apt-get install tuiwall // Ubuntu / Debian
		sudo pacman install tuiwall // Arch
		sudo yum install tuiwall // Amazon

	Enable / Disable header using
		tuiwall enable / disable

	Quick reset (disable & enable) using
		tuiwall reset

	Change presets using
		tuiwall set clock / status

	List installed presets using
		tuiwall list

	Status report using
		tuiwall status

	Install presets using
		tuiwall install	<repo url>

	Uninstall presets using
		tuiwall uninstall <preset>

	Edit / create presets using
		tuiwall preset <new | edit | path> <preset>

	Upload presets using
		tuiwall upload <preset> <repo url>

### WARNING : Vet every preset yourself via the editor prior to running. 

