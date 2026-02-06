#!/usr/bin/env python3
"""
Convert pseudo-transparent checkerboard assets into true-alpha PNG files.

Typical usage:
  uv run --with pillow python scripts/make_transparent.py \
    --input tmp/imagegen/assets/*.jpeg \
    --out-dir tmp/imagegen/assets-alpha
"""

from __future__ import annotations

import argparse
import glob
import os
from collections import Counter
from typing import Iterable, List, Sequence, Tuple

from PIL import Image


RGB = Tuple[int, int, int]


def parse_hex_color(value: str) -> RGB:
    s = value.strip().lstrip("#")
    if len(s) != 6:
        raise ValueError(f"invalid color: {value}")
    return (int(s[0:2], 16), int(s[2:4], 16), int(s[4:6], 16))


def color_dist(a: RGB, b: RGB) -> int:
    return abs(a[0] - b[0]) + abs(a[1] - b[1]) + abs(a[2] - b[2])


def clamp(v: int, lo: int, hi: int) -> int:
    return max(lo, min(v, hi))


def detect_background_colors(img: Image.Image, quant_step: int = 8) -> Tuple[RGB, RGB]:
    w, h = img.size
    px = img.convert("RGB").load()
    border_colors: Counter[RGB] = Counter()

    def q(c: RGB) -> RGB:
        return tuple(clamp((x // quant_step) * quant_step, 0, 255) for x in c)  # type: ignore[return-value]

    for x in range(w):
        border_colors[q(px[x, 0])] += 1
        border_colors[q(px[x, h - 1])] += 1
    for y in range(h):
        border_colors[q(px[0, y])] += 1
        border_colors[q(px[w - 1, y])] += 1

    most = [c for c, _ in border_colors.most_common(12)]
    if not most:
        return (90, 90, 90), (64, 64, 64)

    c1 = most[0]
    c2 = None
    for c in most[1:]:
        if color_dist(c, c1) >= 24:
            c2 = c
            break
    if c2 is None:
        c2 = (max(c1[0] - 24, 0), max(c1[1] - 24, 0), max(c1[2] - 24, 0))
    return c1, c2


def expand_inputs(inputs: Sequence[str]) -> List[str]:
    out: List[str] = []
    for raw in inputs:
        matches = glob.glob(raw)
        if matches:
            out.extend(matches)
        elif os.path.isfile(raw):
            out.append(raw)
    dedup = sorted(set(out))
    return dedup


def remove_bg(
    img: Image.Image,
    bg1: RGB,
    bg2: RGB,
    tol: int,
    soft_tol: int,
) -> Image.Image:
    src = img.convert("RGBA")
    data = src.load()
    w, h = src.size
    out = Image.new("RGBA", (w, h))
    dst = out.load()

    hard = max(0, tol)
    soft = max(hard + 1, soft_tol)

    for y in range(h):
        for x in range(w):
            r, g, b, a = data[x, y]
            d = min(color_dist((r, g, b), bg1), color_dist((r, g, b), bg2))
            if d <= hard:
                dst[x, y] = (r, g, b, 0)
                continue
            if d >= soft:
                dst[x, y] = (r, g, b, a)
                continue

            # Feather alpha between [hard, soft] to reduce edge halos.
            ratio = float(d - hard) / float(soft - hard)
            aa = int(a * ratio)
            dst[x, y] = (r, g, b, aa)

    return out


def convert_files(
    files: Iterable[str],
    out_dir: str,
    auto_detect: bool,
    bg1: RGB,
    bg2: RGB,
    tol: int,
    soft_tol: int,
) -> None:
    os.makedirs(out_dir, exist_ok=True)
    for path in files:
        img = Image.open(path)
        c1, c2 = (detect_background_colors(img) if auto_detect else (bg1, bg2))
        alpha_img = remove_bg(img, c1, c2, tol=tol, soft_tol=soft_tol)
        base = os.path.splitext(os.path.basename(path))[0]
        out_path = os.path.join(out_dir, f"{base}.png")
        alpha_img.save(out_path, format="PNG")
        print(f"[ok] {path} -> {out_path} (bg1={c1}, bg2={c2})")


def main() -> None:
    p = argparse.ArgumentParser(description="Convert pseudo-transparent background to real alpha PNG.")
    p.add_argument("--input", nargs="+", required=True, help="Input file(s) or glob(s).")
    p.add_argument("--out-dir", required=True, help="Output directory for PNG files.")
    p.add_argument("--bg1", default="#5A5A5A", help="Background color 1 (hex).")
    p.add_argument("--bg2", default="#3F3F3F", help="Background color 2 (hex).")
    p.add_argument("--tol", type=int, default=24, help="Hard transparency threshold (L1 distance).")
    p.add_argument("--soft-tol", type=int, default=48, help="Soft edge threshold for feathering.")
    p.add_argument("--auto-detect", action="store_true", help="Detect two checkerboard colors from borders.")
    args = p.parse_args()

    files = expand_inputs(args.input)
    if not files:
        raise SystemExit("no input files matched")

    bg1 = parse_hex_color(args.bg1)
    bg2 = parse_hex_color(args.bg2)
    convert_files(
        files=files,
        out_dir=args.out_dir,
        auto_detect=args.auto_detect,
        bg1=bg1,
        bg2=bg2,
        tol=args.tol,
        soft_tol=args.soft_tol,
    )


if __name__ == "__main__":
    main()
