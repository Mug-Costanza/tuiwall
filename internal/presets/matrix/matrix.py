#!/usr/bin/env python3
# vim: set expandtab shiftwidth=4 softtabstop=4 tabstop=4:
# -*- mode: python; indent-tabs-mode: nil; python-indent-offset: 4; -*-

# --- TUIWALL METADATA ---
# Name: Matrix
# Author: Mug
# Description: Digital matrix rain
# Category: Ambiance
# ------------------------

import curses
import random
import time

# Half-width Katakana range + some numbers/symbols
KATAKANA = "".join(chr(i) for i in range(0xff66, 0xff9d))
CHARS = KATAKANA + "0123456789$+-<>[]"

def main(stdscr):
    # Ensure terminal handles UTF-8 characters
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    curses.nonl()

    # Color definitions (Dark -> Bright -> White head)
    try:
        curses.init_pair(1, 22, -1)   # Dark green
        curses.init_pair(2, 28, -1)   # Mid green
        curses.init_pair(3, 46, -1)   # Bright green
        curses.init_pair(4, 15, -1)   # White
    except curses.error:
        curses.init_pair(1, curses.COLOR_GREEN, -1)
        curses.init_pair(4, curses.COLOR_WHITE, -1)

    stdscr.nodelay(True)
    stdscr.timeout(80)
    stdscr.clearok(True)

    columns = {}

    while True:
        h, w = stdscr.getmaxyx()
        top = 2
        sky_h = max(1, h - top)

        # Header logic similar to rain.py
        now = time.strftime("%H:%M:%S")
        date = time.strftime("%a %b %d")
        try:
            stdscr.addstr(0, 0, ("  tuiwall  " + now).ljust(w)[:w], curses.A_BOLD)
            stdscr.addstr(1, 0, ("  " + date).ljust(w)[:w])
        except curses.error:
            pass

        # Spawn new streams based on width
        if len(columns) < w // 3:
            x = random.randint(0, w - 1)
            if x not in columns:
                columns[x] = {
                    'y': float(-random.randint(0, sky_h)),
                    'speed': random.uniform(0.3, 1.0),
                    'len': random.randint(8, sky_h),
                }

        for x in list(columns.keys()):
            col = columns[x]
            curr_y = int(col['y'])

            # Draw the trail
            for i in range(col['len']):
                draw_y = curr_y - i
                if 0 <= draw_y < sky_h:
                    if i == 0:
                        attr = curses.color_pair(4) | curses.A_BOLD
                    elif i < 3:
                        attr = curses.color_pair(3) | curses.A_BOLD
                    else:
                        attr = curses.color_pair(1 if i > col['len']*0.7 else 2)
                    
                    # Random character flicker logic
                    char = random.choice(CHARS)
                    try:
                        stdscr.addstr(top + draw_y, x, char, attr)
                    except curses.error:
                        pass
                
                # Clean up tail
                erase_y = curr_y - col['len']
                if 0 <= erase_y < sky_h:
                    try:
                        stdscr.addstr(top + erase_y, x, " ")
                    except curses.error:
                        pass

            col['y'] += col['speed']
            if int(col['y']) - col['len'] > sky_h:
                del columns[x]

        stdscr.refresh()
        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)
