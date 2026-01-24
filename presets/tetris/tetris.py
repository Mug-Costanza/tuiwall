#!/usr/bin/env python3
import curses
import random
import time

# Tetrominoes as 4x4 bitmaps (list of rotation states)
# Each state is a list of (x,y) blocks relative to a 4x4 box.
TETROS = {
    "I": [
        [(0,1),(1,1),(2,1),(3,1)],
        [(2,0),(2,1),(2,2),(2,3)],
    ],
    "O": [
        [(1,1),(2,1),(1,2),(2,2)],
    ],
    "T": [
        [(1,1),(0,2),(1,2),(2,2)],
        [(1,1),(1,2),(2,2),(1,3)],
        [(0,2),(1,2),(2,2),(1,3)],
        [(1,1),(0,2),(1,2),(1,3)],
    ],
    "S": [
        [(1,1),(2,1),(0,2),(1,2)],
        [(1,1),(1,2),(2,2),(2,3)],
    ],
    "Z": [
        [(0,1),(1,1),(1,2),(2,2)],
        [(2,1),(1,2),(2,2),(1,3)],
    ],
    "J": [
        [(0,1),(0,2),(1,2),(2,2)],
        [(1,1),(2,1),(1,2),(1,3)],
        [(0,2),(1,2),(2,2),(2,3)],
        [(1,1),(1,2),(0,3),(1,3)],
    ],
    "L": [
        [(2,1),(0,2),(1,2),(2,2)],
        [(1,1),(1,2),(1,3),(2,3)],
        [(0,2),(1,2),(2,2),(0,3)],
        [(0,1),(1,1),(1,2),(1,3)],
    ],
}

PIECE_ORDER = list(TETROS.keys())

def blocks_for(piece, rot):
    states = TETROS[piece]
    return states[rot % len(states)]

def can_place(board, w, h, piece, rot, px, py):
    for bx, by in blocks_for(piece, rot):
        x = px + bx
        y = py + by
        if x < 0 or x >= w or y < 0 or y >= h:
            return False
        if board[y][x] != 0:
            return False
    return True

def lock_piece(board, piece, rot, px, py, color_id):
    h = len(board)
    w = len(board[0]) if h else 0
    for bx, by in blocks_for(piece, rot):
        x = px + bx
        y = py + by
        if 0 <= x < w and 0 <= y < h:
            board[y][x] = color_id

def draw_cell(stdscr, y, x, val, top):
    # single-cell rendering; using "█" keeps it chunky even at small sizes
    if val == 0:
        ch = " "
        attr = 0
    else:
        ch = "█"
        attr = curses.color_pair(val) | curses.A_BOLD
    try:
        stdscr.addstr(top + y, x, ch, attr)
    except curses.error:
        pass

def init_colors():
    # Best-effort 256-color ids; still fine if terminal maps differently.
    # 1..7 are piece colors (avoid 0).
    return [
        39,   # cyan-ish
        220,  # yellow-ish
        201,  # magenta-ish
        82,   # green-ish
        196,  # red-ish
        27,   # blue-ish
        208,  # orange-ish
    ]

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    palette = init_colors()
    for i, c in enumerate(palette, start=1):
        try:
            curses.init_pair(i, c, -1)
        except curses.error:
            # fallback if init_pair fails
            curses.init_pair(i, 15, -1)

    stdscr.nodelay(True)
    stdscr.timeout(30)

    # Tuning knobs (frozen vibe)
    DROP_EVERY_SECONDS = 0.35   # bigger => slower fall
    SOFT_DRIFT_CHANCE  = 0.12   # gentle sideways wobble
    ROTATE_CHANCE      = 0.10   # occasional mid-air rotate
    RESET_PAUSE        = 0.6    # pause before clearing when topped out

    board = None
    last_drop = time.time()

    cur_piece = None
    cur_rot = 0
    cur_x = 0
    cur_y = 0
    cur_color = 1

    def reset_board(sea_h, w):
        return [[0 for _ in range(w)] for _ in range(sea_h)]

    def spawn_piece(board, w, h):
        nonlocal cur_piece, cur_rot, cur_x, cur_y, cur_color
        cur_piece = random.choice(PIECE_ORDER)
        cur_rot = random.randint(0, 3)
        cur_y = -1  # start slightly above
        # spawn centered-ish, then clamp
        cur_x = max(0, min(w - 4, w // 2 - 2 + random.randint(-w//6 if w>6 else 0, w//6 if w>6 else 0)))
        cur_color = random.randint(1, 7)

        # If spawn blocked, we’ll handle “topped out” outside.
        return can_place(board, w, h, cur_piece, cur_rot, cur_x, max(cur_y, 0))

    while True:
        h, w = stdscr.getmaxyx()
        top = 2
        play_h = max(1, h - top)
        play_w = max(1, w)

        if board is None or len(board) != play_h or len(board[0]) != play_w:
            board = reset_board(play_h, play_w)
            spawn_piece(board, play_w, play_h)

        # Header (matches your tuiwall look)
        now = time.strftime("%H:%M:%S")
        date = time.strftime("%a %b %d")
        try:
            stdscr.addstr(0, 0, ("  tuiwall  " + now).ljust(play_w)[:play_w], curses.A_BOLD)
            stdscr.addstr(1, 0, ("  " + date).ljust(play_w)[:play_w])
        except curses.error:
            pass

        # Occasionally nudge piece sideways / rotate (very gently)
        if cur_piece is not None and random.random() < SOFT_DRIFT_CHANCE:
            dx = random.choice([-1, 1])
            if can_place(board, play_w, play_h, cur_piece, cur_rot, cur_x + dx, max(cur_y, 0)):
                cur_x += dx

        if cur_piece is not None and random.random() < ROTATE_CHANCE:
            nr = (cur_rot + 1) % 4
            if can_place(board, play_w, play_h, cur_piece, nr, cur_x, max(cur_y, 0)):
                cur_rot = nr

        # Drop step
        t = time.time()
        if t - last_drop >= DROP_EVERY_SECONDS:
            last_drop = t

            # Try to move down
            ny = cur_y + 1
            if can_place(board, play_w, play_h, cur_piece, cur_rot, cur_x, max(ny, 0)):
                cur_y = ny
            else:
                # Lock at current position (clamp y to 0 so it "lands" if it was above)
                lock_piece(board, cur_piece, cur_rot, cur_x, max(cur_y, 0), cur_color)

                # Check topped out: any blocks in top rows
                topped = any(board[0][x] != 0 for x in range(play_w))
                if topped:
                    stdscr.refresh()
                    time.sleep(RESET_PAUSE)
                    board = reset_board(play_h, play_w)

                spawn_piece(board, play_w, play_h)

        # Draw board
        for yy in range(play_h):
            for xx in range(play_w):
                draw_cell(stdscr, yy, xx, board[yy][xx], top)

        # Draw current falling piece on top
        if cur_piece is not None:
            for bx, by in blocks_for(cur_piece, cur_rot):
                x = cur_x + bx
                y = cur_y + by
                if 0 <= x < play_w and 0 <= y < play_h:
                    draw_cell(stdscr, y, x, cur_color, top)

        stdscr.refresh()

        # Exit on keypress (helpful in the pane)
        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)

