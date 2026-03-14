#!/usr/bin/env python3
# vim: set expandtab shiftwidth=4 softtabstop=4 tabstop=4:
# -*- mode: python; indent-tabs-mode: nil; python-indent-offset: 4; -*-

# --- TUIWALL METADATA ---
# Name: Stats
# Author: Mug
# Description: Displays CPU, RAM, and Disk usage
# Category: System
# ------------------------

import curses
import time
import psutil
import shutil

def draw_bar(stdscr, y, x, label, percent, width, color_pair):
    """Draws a labeled horizontal bar graph."""
    label_str = f"{label}:".ljust(10)
    bar_width = width - 20
    filled_chars = int((percent / 100) * bar_width)
    
    # Construct the bar: [######      ] 75%
    bar = "[" + ("#" * filled_chars) + (" " * (bar_width - filled_chars)) + "]"
    stat_str = f" {percent:>5.1f}%"
    
    try:
        stdscr.addstr(y, x, label_str)
        stdscr.addstr(y, x + 10, bar, color_pair)
        stdscr.addstr(y, x + 10 + len(bar), stat_str)
    except curses.error:
        pass

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    # Colors for different levels of usage
    curses.init_pair(1, curses.COLOR_GREEN, -1)  # Low/Normal
    curses.init_pair(2, curses.COLOR_YELLOW, -1) # Warning
    curses.init_pair(3, curses.COLOR_RED, -1)    # Critical

    stdscr.nodelay(True)
    stdscr.timeout(1000) # Update every second
    stdscr.clearok(True)

    while True:
        stdscr.erase()
        h, w = stdscr.getmaxyx()
        top = 2
        
        # Header (Matches the tuiwall look)
        now = time.strftime("%H:%M:%S")
        date = time.strftime("%a %b %d")
        try:
            stdscr.addstr(0, 0, ("  tuiwall  " + now).ljust(w)[:w], curses.A_BOLD)
            stdscr.addstr(1, 0, ("  " + date).ljust(w)[:w])
        except curses.error:
            pass

        # Gather Stats
        cpu_pct = psutil.cpu_percent()
        mem = psutil.virtual_memory()
        disk = psutil.disk_usage('/')
        
        # Determine bar colors based on intensity
        def get_color(pct):
            if pct < 60: return curses.color_pair(1)
            if pct < 85: return curses.color_pair(2)
            return curses.color_pair(3)

        # Layout metrics
        metrics = [
            ("CPU", cpu_pct),
            ("RAM", mem.percent),
            ("DISK", disk.percent)
        ]

        # Draw Bars
        for i, (label, pct) in enumerate(metrics):
            draw_bar(stdscr, top + 2 + (i * 2), 0, label, pct, min(w - 8, 60), get_color(pct))

        # Additional Info below bars
        try:
            info_y = top + 2 + (len(metrics) * 2) + 1
            load_avg = " / ".join(map(str, psutil.getloadavg()))
            stdscr.addstr(info_y, 4, f"Load Avg: {load_avg}")
            stdscr.addstr(info_y + 1, 4, f"Uptime:   {time.strftime('%Hh %Mm', time.gmtime(time.time() - psutil.boot_time()))}")
        except curses.error:
            pass

        stdscr.clearok(True)
        stdscr.refresh()

        # Exit on keypress
        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)
