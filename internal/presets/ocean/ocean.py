#!/usr/bin/env python3
# vim: set expandtab shiftwidth=4 softtabstop=4 tabstop=4:
# -*- mode: python; indent-tabs-mode: nil; python-indent-offset: 4; -*-

# --- TUIWALL METADATA ---
# Name: Ocean
# Author: Mug
# Description: Ocean waves
# Category: Ambiance
# ------------------------

import curses
import math
import time
import random

WAVE_CHARS = ["~", "≈", "∼"]
FOAM_CHARS = ["·", ".", "°"]

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    curses.nonl()

    # Ocean-ish colors (best-effort across terminals)
    curses.init_pair(1, 24, -1)   # deep blue
    curses.init_pair(2, 31, -1)   # blue
    curses.init_pair(3, 38, -1)   # cyan
    curses.init_pair(4, 15, -1)   # white foam

    stdscr.nodelay(True)
    stdscr.timeout(80)
    stdscr.clearok(True)

    phase = 0.0
    last_time = time.time()

    while True:
        h, w = stdscr.getmaxyx()
        top = 2
        sea_h = max(1, h - top)

        # Header
        now = time.strftime("%H:%M:%S")
        date = time.strftime("%a %b %d")
        stdscr.addstr(0, 0, ("  tuiwall  " + now).ljust(w)[:w], curses.A_BOLD)
        stdscr.addstr(1, 0, ("  " + date).ljust(w)[:w])

        t = time.time()
        dt = t - last_time
        last_time = t
        phase += dt * 0.8   # wave speed

        # Draw ocean
        for y in range(sea_h):
            depth = y / sea_h
            amp = 1.5 + depth * 2.5
            freq = 0.15 + depth * 0.25
            color = (
                1 if depth > 0.66 else
                2 if depth > 0.33 else
                3
            )

            for x in range(w):
                wave_y = math.sin(x * freq + phase + depth * 3)
                crest = amp * wave_y

                if abs(crest) < 0.4:
                    ch = " "
                else:
                    ch = random.choice(WAVE_CHARS)

                # Occasional foam
                if crest > amp * 0.85 and random.random() < 0.03:
                    ch = random.choice(FOAM_CHARS)
                    attr = curses.color_pair(4) | curses.A_BOLD
                else:
                    attr = curses.color_pair(color)

                try:
                    stdscr.addstr(top + y, x, ch, attr)
                except curses.error:
                    pass

        # stdscr.clearok(True)
        stdscr.refresh()

        # Exit on keypress
        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)

