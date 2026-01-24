#!/usr/bin/env python3
import curses
import math
import time
import random

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    stdscr.nodelay(True)
    stdscr.timeout(80)

    while True:
        h, w = stdscr.getmaxyx()
        top = 2
        h = max(1, h - top)
        w = max(1, w)

        # Header
        now = time.strftime("%H:%M:%S")
        date = time.strftime("%a %b %d")
    
        try:
            stdscr.addstr(0, 0, ("  tuiwall  " + now).ljust(w)[:w], curses.A_BOLD)
            stdscr.addstr(1, 0, ("  " + date).ljust(w)[:w])
        except curses.error:
            pass


        stdscr.refresh()

        # Exit on keypress
        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)


