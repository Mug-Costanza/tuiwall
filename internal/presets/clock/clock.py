#!/usr/bin/env python3
# vim: set expandtab shiftwidth=4 softtabstop=4 tabstop=4:
# -*- mode: python; indent-tabs-mode: nil; python-indent-offset: 4; -*-

# --- TUIWALL METADATA ---
# Name: Clock 
# Author: Mug
# Description: Digital clock
# Category: Dashboard
# ------------------------

import curses
import time
import sys

# ASCII patterns for numbers 0-9 and colon
DIGITS = {
    '0': ["███", "█ █", "█ █", "█ █", "███"],
    '1': ["  █", "  █", "  █", "  █", "  █"],
    '2': ["███", "  █", "███", "█  ", "███"],
    '3': ["███", "  █", "███", "  █", "███"],
    '4': ["█ █", "█ █", "███", "  █", "  █"],
    '5': ["███", "█  ", "███", "  █", "███"],
    '6': ["███", "█  ", "███", "█ █", "███"],
    '7': ["███", "  █", "  █", "  █", "  █"],
    '8': ["███", "█ █", "███", "█ █", "███"],
    '9': ["███", "█ █", "███", "  █", "███"],
    ':': ["   ", " █ ", "   ", " █ ", "   "],
}

def main(stdscr):
    # Setup curses
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    curses.nonl()
    
    # Pair 1: Cyan highlight for the big clock
    curses.init_pair(1, curses.COLOR_CYAN, -1)
    
    stdscr.nodelay(True)
    stdscr.timeout(250) # Refresh rate matching your Go ticker

     # Force full repaint each refresh (helps if the mirror clears every frame)
    stdscr.clearok(True)

    while True:
        # stdscr.erase() can sometimes leave artifacts in PTYs; 
        # explicit padding is more reliable.
        stdscr.erase()
        h, w = stdscr.getmaxyx()
        
        # 1. Header Logic
        now_str = time.strftime("%H:%M:%S")
        date_str = time.strftime("%a %b %d")
        
        try:
            # ljust(w) ensures the entire background of the line is overwritten
            stdscr.addstr(0, 0, ("  tuiwall  " + now_str).ljust(w)[:w], curses.A_BOLD)
            stdscr.addstr(1, 0, ("  " + date_str).ljust(w)[:w])
        except curses.error:
            pass

        # 2. Big Clock Logic
        big_time = time.strftime("%H:%M:%S")
        digit_h = 5
        digit_w = 4 
        total_w = len(big_time) * digit_w - 1
        
        start_y = (h // 2) - (digit_h // 2)
        # Ensure we don't start at a negative X if the window is tiny
        start_x = max(0, (w // 2) - (total_w // 2))

        if h > 6 and w > total_w:
            for row in range(digit_h):
                line_content = ""
                for char in big_time:
                    line_content += DIGITS[char][row] + " "
                
                try:
                    # Construct a full-width string:
                    # [Padding Spaces][Clock Content][Remainder of Screen Padded]
                    full_line = (" " * start_x) + line_content
                    padded_line = full_line.ljust(w)
                    
                    stdscr.addstr(start_y + row, 0, padded_line[:w], curses.color_pair(1) | curses.A_BOLD)
                except curses.error:
                    pass
        else:
            msg = "Terminal too small".center(w)
            try:
                stdscr.addstr(h // 2, 0, msg[:w])
            except curses.error:
                pass
        stdscr.clearok(True)
        stdscr.refresh()

        # Exit on keypress
        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    try:
        curses.wrapper(main)
    except KeyboardInterrupt:
        sys.exit(0)
