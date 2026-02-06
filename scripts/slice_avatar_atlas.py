#!/usr/bin/env python3
"""Slice avatar atlas into individual PNG files."""

from __future__ import annotations

import argparse
from collections import deque
from pathlib import Path

from PIL import Image


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Slice avatar atlas to per-avatar files.")
    p.add_argument(
        "--input",
        default="web/src/assets/replay-pixel/avatars_pack.png",
        help="Avatar atlas input PNG path.",
    )
    p.add_argument(
        "--out-dir",
        default="web/src/assets/replay-pixel/avatars",
        help="Output directory for sliced avatars.",
    )
    p.add_argument("--cols", type=int, default=4, help="Atlas columns.")
    p.add_argument("--rows", type=int, default=2, help="Atlas rows.")
    p.add_argument("--start-row", type=int, default=0, help="Start row index (0-based).")
    p.add_argument("--row-count", type=int, default=0, help="How many rows to export (0 means until end).")
    p.add_argument("--pad-x", type=int, default=0, help="Horizontal padding for each cell crop.")
    p.add_argument("--pad-y", type=int, default=0, help="Vertical padding for each cell crop.")
    p.add_argument(
        "--crop-mode",
        choices=("full", "portrait"),
        default="portrait",
        help="full: keep full cell, portrait: keep upper portrait area and drop lower label area.",
    )
    p.add_argument(
        "--portrait-ratio",
        type=float,
        default=0.82,
        help="When crop-mode=portrait, keep only the top ratio of each tile before alpha-trim.",
    )
    p.add_argument(
        "--trim-alpha",
        action="store_true",
        default=True,
        help="Trim each tile to non-transparent bounds.",
    )
    p.add_argument(
        "--no-trim-alpha",
        action="store_false",
        dest="trim_alpha",
        help="Disable alpha trimming.",
    )
    p.add_argument(
        "--out-size",
        type=int,
        default=0,
        help="If > 0, resize output to out-size x out-size with NEAREST.",
    )
    p.add_argument(
        "--clean",
        action="store_true",
        help="Delete existing avatar_*.png in out-dir before writing new slices.",
    )
    p.add_argument(
        "--key-bg",
        action="store_true",
        default=True,
        help="Try to remove checkerboard background by turning border-matching colors transparent.",
    )
    p.add_argument(
        "--no-key-bg",
        action="store_false",
        dest="key_bg",
        help="Disable checkerboard background keying.",
    )
    p.add_argument(
        "--bg-tolerance",
        type=int,
        default=16,
        help="RGB tolerance used with --key-bg.",
    )
    return p.parse_args()


def trim_alpha(tile: Image.Image) -> Image.Image:
    alpha = tile.getchannel("A")
    bbox = alpha.getbbox()
    if not bbox:
        return tile
    return tile.crop(bbox)


def crop_portrait(tile: Image.Image, ratio: float) -> Image.Image:
    """Crop inner avatar frame area and exclude outer checkerboard + label band."""
    w, h = tile.size
    if ratio <= 0 or ratio > 1:
        ratio = 0.78
    # Tuned for current 4x2 avatar atlas layout.
    x0 = int(w * 0.13)
    x1 = int(w * 0.87)
    y0 = int(h * 0.11)
    y1 = int(h * ratio)
    if x0 >= x1 or y0 >= y1:
        return trim_alpha(tile)
    cropped = tile.crop((x0, y0, x1, y1))
    return trim_alpha(cropped)


def _close_rgb(a: tuple[int, int, int], b: tuple[int, int, int], tol: int) -> bool:
    return abs(a[0] - b[0]) <= tol and abs(a[1] - b[1]) <= tol and abs(a[2] - b[2]) <= tol


def key_checkerboard_bg(tile: Image.Image, tolerance: int) -> Image.Image:
    """Remove only outer checkerboard background via corner-seeded flood fill."""
    rgba = tile.copy().convert("RGBA")
    w, h = rgba.size
    px = rgba.load()
    corners = [(0, 0), (w - 1, 0), (0, h - 1), (w - 1, h - 1)]
    bg_seeds = [px[x, y][:3] for x, y in corners]
    q = deque(corners)
    seen = set(corners)

    while q:
        x, y = q.popleft()
        r, g, b, a = px[x, y]
        if a == 0:
            continue
        rgb = (r, g, b)
        if any(_close_rgb(rgb, bg, tolerance) for bg in bg_seeds):
            px[x, y] = (r, g, b, 0)
            if x > 0 and (x - 1, y) not in seen:
                seen.add((x - 1, y))
                q.append((x - 1, y))
            if x + 1 < w and (x + 1, y) not in seen:
                seen.add((x + 1, y))
                q.append((x + 1, y))
            if y > 0 and (x, y - 1) not in seen:
                seen.add((x, y - 1))
                q.append((x, y - 1))
            if y + 1 < h and (x, y + 1) not in seen:
                seen.add((x, y + 1))
                q.append((x, y + 1))
    return rgba


def main() -> None:
    args = parse_args()
    src = Path(args.input)
    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    if args.clean:
        for p in out_dir.glob("avatar_*.png"):
            p.unlink()

    if not src.exists():
        raise SystemExit(f"input not found: {src}")

    atlas = Image.open(src).convert("RGBA")
    aw, ah = atlas.size
    if args.cols < 1 or args.rows < 1:
        raise SystemExit("cols/rows must be >= 1")
    cw = aw // args.cols
    ch = ah // args.rows

    start_row = max(0, args.start_row)
    end_row = args.rows if args.row_count <= 0 else min(args.rows, start_row + args.row_count)

    idx = 1
    for r in range(start_row, end_row):
        for c in range(args.cols):
            x0 = c * cw + args.pad_x
            y0 = r * ch + args.pad_y
            x1 = (c + 1) * cw - args.pad_x
            y1 = (r + 1) * ch - args.pad_y

            if x0 >= x1 or y0 >= y1:
                raise SystemExit("invalid pad values produce empty crop")

            tile = atlas.crop((x0, y0, x1, y1))
            if args.key_bg:
                tile = key_checkerboard_bg(tile, max(0, args.bg_tolerance))
            if args.crop_mode == "portrait":
                tile = crop_portrait(tile, args.portrait_ratio)
            if args.trim_alpha:
                tile = trim_alpha(tile)
            if args.out_size > 0 and tile.size != (args.out_size, args.out_size):
                tile = tile.resize((args.out_size, args.out_size), Image.Resampling.NEAREST)
            out = out_dir / f"avatar_{idx:02d}.png"
            tile.save(out, format="PNG")
            idx += 1

    print(f"[ok] sliced {idx - 1} avatars -> {out_dir}")


if __name__ == "__main__":
    main()
