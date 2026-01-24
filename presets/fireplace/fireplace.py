#!/usr/bin/env python3
import curses, random, time, os, signal

CHARS = [" ", ".", ":", "^", "*", "x", "s", "S", "#", "$"]

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    # color pairs: (fg, bg)
    curses.init_pair(1, 0, -1)   # black/none
    curses.init_pair(2, 1, -1)   # red
    curses.init_pair(3, 3, -1)   # yellow
    curses.init_pair(4, 4, -1)   # blue-ish (you can tweak)

    stdscr.nodelay(True)
    stdscr.timeout(30)

    while True:
        h, w = stdscr.getmaxyx()
        # Reserve top 2 lines for clock; fire uses the rest
        top = 2
        fh = max(1, h - top)
        fw = max(1, w)
        size = fw * fh

        # buffer includes +fw+1 padding like your original
        b = [0] * (size + fw + 1)

        while True:
            # handle resize
            nh, nw = stdscr.getmaxyx()
            if nh != h or nw != w:
                break  # rebuild buffers

	    signal.signal(signal.SIGWINCH, handle_resize)

            # clock header (2 lines)
            now = time.strftime("%H:%M:%S")
            date = time.strftime("%a %b %d")
            stdscr.addstr(0, 0, ("  " + now).ljust(w)[:w], curses.A_BOLD)
            stdscr.addstr(1, 0, ("  " + date).ljust(w)[:w])

            # seed heat on bottom row of fire area
            seeds = max(1, fw // 9)
            base_y = top + (fh - 1)
            for _ in range(seeds):
                x = int(random.random() * fw)
                b[x + fw * (fh - 1)] = 65

            # propagate + draw
            for i in range(size):
                # average neighbors
                b[i] = int((b[i] + b[i+1] + b[i+fw] + b[i+fw+1]) / 4)

                color = 4 if b[i] > 15 else (3 if b[i] > 9 else (2 if b[i] > 4 else 1))
                ch = CHARS[9 if b[i] > 9 else b[i]]

                y = top + (i // fw)
                x = i % fw

                if y < h and x < w:
                    try:
                        stdscr.addstr(y, x, ch, curses.color_pair(color) | curses.A_BOLD)
                    except curses.error:
                        pass

            stdscr.refresh()

            # press any key to exit (pane will usually be killed by tuiwall disable)
            if stdscr.getch() != -1:
                return

def run():
    curses.wrapper(main)

if __name__ == "__main__":
    run()

