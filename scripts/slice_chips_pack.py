#!/usr/bin/env python3
"""Slice chips_pack atlas into individual chip PNG tiles.

Output naming model:
- scan atlas left-to-right, top-to-bottom
- every consecutive 4 chips are one color group
- within each 4-chip group, unit grows from 1 to 4
- filename: chip_color_XX_unit_XX.png
"""

from __future__ import annotations

import argparse
from pathlib import Path

from PIL import Image


def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Slice chips atlas to per-chip files.")
    p.add_argument("--input", default="web/src/assets/replay-pixel/chips_pack.png")
    p.add_argument("--out-dir", default="web/src/assets/replay-pixel/chips")
    p.add_argument("--cols", type=int, default=8)
    p.add_argument("--rows", type=int, default=4)
    p.add_argument("--pad-x", type=int, default=0)
    p.add_argument("--pad-y", type=int, default=0)
    p.add_argument("--trim-alpha", action="store_true", default=True)
    p.add_argument("--no-trim-alpha", action="store_false", dest="trim_alpha")
    p.add_argument("--out-size", type=int, default=0, help="If >0, normalize each tile to square out-size.")
    p.add_argument("--clean", action="store_true", help="Delete existing chip_*.png in out-dir before writing new slices.")
    return p.parse_args()


def trim_alpha(tile: Image.Image) -> Image.Image:
    alpha = tile.getchannel("A")
    bbox = alpha.getbbox()
    if not bbox:
        return tile
    return tile.crop(bbox)


def main() -> None:
    args = parse_args()
    src = Path(args.input)
    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    if args.clean:
        for p in out_dir.glob("chip_*.png"):
            p.unlink()

    if not src.exists():
        raise SystemExit(f"input not found: {src}")
    if args.cols < 1 or args.rows < 1:
        raise SystemExit("cols/rows must be >= 1")

    atlas = Image.open(src).convert("RGBA")
    aw, ah = atlas.size
    cw = aw // args.cols
    ch = ah // args.rows

    idx = 1
    for r in range(args.rows):
        for c in range(args.cols):
            x0 = c * cw + args.pad_x
            y0 = r * ch + args.pad_y
            x1 = (c + 1) * cw - args.pad_x
            y1 = (r + 1) * ch - args.pad_y
            if x0 >= x1 or y0 >= y1:
                raise SystemExit("invalid pad values produce empty crop")

            tile = atlas.crop((x0, y0, x1, y1))
            if args.trim_alpha:
                tile = trim_alpha(tile)
            if args.out_size > 0 and tile.size != (args.out_size, args.out_size):
                tile = tile.resize((args.out_size, args.out_size), Image.Resampling.NEAREST)

            # Row-major sequence index across the atlas.
            seq = r * args.cols + c
            # Every 4 consecutive sprites represent one color.
            color_id = (seq // 4) + 1
            # Unit grows within the 4-sprite color group.
            unit_id = (seq % 4) + 1
            out = out_dir / f"chip_color_{color_id:02d}_unit_{unit_id:02d}.png"
            tile.save(out, format="PNG")
            idx += 1

    print(f"[ok] sliced {idx - 1} chips -> {out_dir}")


if __name__ == "__main__":
    main()
