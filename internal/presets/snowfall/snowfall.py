#!/usr/bin/env python3
# vim: set expandtab shiftwidth=4 softtabstop=4 tabstop=4:
# -*- mode: python; indent-tabs-mode: nil; python-indent-offset: 4; -*-

# --- TUIWALL METADATA ---
# Name: Snowfall
# Author: Mug
# Description: Falling snow
# Category: Ambiance
# ------------------------

import curses, random, time

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    curses.nonl()

    curses.init_pair(1, 15, -1)  # white
    curses.init_pair(2, 7, -1)   # light gray
    curses.init_pair(3, 8, -1)   # darker gray

    stdscr.nodelay(True)
    stdscr.timeout(60) 
    stdscr.clearok(True)

    flakes = []
    last_spawn = 0.0

    SPAWN_INTERVAL = 0.06   # lower = more dense (try 0.04–0.10)
    SPAWN_CHANCE  = 0.95    # chance to spawn on each interval
    SPAWN_BURST   = (1, 3)  # number of flakes to add per spawn
    MAX_FLAKES    = 900     # safety cap (scales with width/height)

    while True:
        h, w = stdscr.getmaxyx()
        top = 2
        sky_h = max(1, h - top)

        # Header
        now = time.strftime("%H:%M:%S")
        date = time.strftime("%a %b %d")
        stdscr.addstr(0, 0, ("  tuiwall  " + now).ljust(w)[:w], curses.A_BOLD)
        stdscr.addstr(1, 0, ("  " + date).ljust(w)[:w])

        t = time.time()

        # Spawn new flakes (denser)
        if t - last_spawn > SPAWN_INTERVAL and len(flakes) < MAX_FLAKES:
            last_spawn = t
            if random.random() < SPAWN_CHANCE:
                burst = random.randint(SPAWN_BURST[0], SPAWN_BURST[1])
                for _ in range(burst):
                    x = random.randint(0, w - 1)
                    dx = random.choice([-1, 0, 0, 0, 1])   # mostly straight down
                    speed = random.choice([1, 1, 1, 2])    # most flakes update every tick
                    ch = random.choice([".", "·", "*"])
                    col = random.choice([1, 2, 2, 3])
                    flakes.append([x, -1, dx, speed, ch, col, 0, None])  # prev stored last

        new_flakes = []
        for f in flakes:
            x, y, dx, speed, ch, col, tick, prev = f

            # erase previous position (NO TRAIL)
            if prev is not None:
                px, py = prev
                if 0 <= py < sky_h and 0 <= px < w:
                    try:
                        stdscr.addstr(top + py, px, " ")
                    except curses.error:
                        pass

            tick += 1
            if tick >= speed:
                tick = 0
                y += 1
                if random.random() < 0.35:
                    x += dx
                x = max(0, min(w - 1, x))

            # draw at new position
            if 0 <= y < sky_h:
                try:
                    stdscr.addstr(top + y, x, ch, curses.color_pair(col))
                except curses.error:
                    pass
                prev = (x, y)

            if y < sky_h:
                new_flakes.append([x, y, dx, speed, ch, col, tick, prev])

        flakes = new_flakes
        stdscr.clearok(True)
        stdscr.refresh()

        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)

