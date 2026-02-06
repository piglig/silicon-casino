#!/usr/bin/env python3
"""
Slice a poker card atlas (transparent background) into per-card PNG files.

Expected output names:
  AH.png, 2H.png, ..., KH.png
  AD.png, ..., KD.png
  AC.png, ..., KC.png
  AS.png, ..., KS.png
  BACK_1.png, BACK_2.png

Usage:
  uv run --with pillow python scripts/slice_card_atlas.py \
    --input web/src/assets/replay-pixel/cards_deck_atlas.png \
    --out-dir web/src/assets/replay-pixel/cards
"""

from __future__ import annotations

import argparse
from collections import deque
from dataclasses import dataclass
from pathlib import Path
from typing import List, Tuple

from PIL import Image


@dataclass
class Box:
    x0: int
    y0: int
    x1: int
    y1: int

    @property
    def w(self) -> int:
        return self.x1 - self.x0 + 1

    @property
    def h(self) -> int:
        return self.y1 - self.y0 + 1

    @property
    def area(self) -> int:
        return self.w * self.h

    @property
    def cx(self) -> float:
        return (self.x0 + self.x1) / 2.0

    @property
    def cy(self) -> float:
        return (self.y0 + self.y1) / 2.0


RANKS = ["A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"]
SUITS = ["H", "D", "C", "S"]


def find_alpha_components(alpha: Image.Image, threshold: int, min_area: int) -> List[Box]:
    w, h = alpha.size
    pix = alpha.load()
    visited = bytearray(w * h)
    boxes: List[Box] = []

    def idx(x: int, y: int) -> int:
        return y * w + x

    for y in range(h):
        for x in range(w):
            i = idx(x, y)
            if visited[i]:
                continue
            visited[i] = 1
            if pix[x, y] <= threshold:
                continue

            q = deque([(x, y)])
            x0 = x1 = x
            y0 = y1 = y
            count = 0

            while q:
                cx, cy = q.popleft()
                count += 1
                if cx < x0:
                    x0 = cx
                if cx > x1:
                    x1 = cx
                if cy < y0:
                    y0 = cy
                if cy > y1:
                    y1 = cy

                for nx, ny in ((cx - 1, cy), (cx + 1, cy), (cx, cy - 1), (cx, cy + 1)):
                    if nx < 0 or nx >= w or ny < 0 or ny >= h:
                        continue
                    ni = idx(nx, ny)
                    if visited[ni]:
                        continue
                    visited[ni] = 1
                    if pix[nx, ny] > threshold:
                        q.append((nx, ny))

            box = Box(x0, y0, x1, y1)
            if count >= min_area and box.w >= 16 and box.h >= 16:
                boxes.append(box)

    return boxes


def find_white_face_components(rgb: Image.Image, threshold: int, min_area: int) -> List[Box]:
    w, h = rgb.size
    pix = rgb.load()
    visited = bytearray(w * h)
    boxes: List[Box] = []

    def idx(x: int, y: int) -> int:
        return y * w + x

    for y in range(h):
        for x in range(w):
            i = idx(x, y)
            if visited[i]:
                continue
            visited[i] = 1
            r, g, b = pix[x, y]
            if not (r >= threshold and g >= threshold and b >= threshold):
                continue

            q = deque([(x, y)])
            x0 = x1 = x
            y0 = y1 = y
            count = 0

            while q:
                cx, cy = q.popleft()
                count += 1
                if cx < x0:
                    x0 = cx
                if cx > x1:
                    x1 = cx
                if cy < y0:
                    y0 = cy
                if cy > y1:
                    y1 = cy

                for nx, ny in ((cx - 1, cy), (cx + 1, cy), (cx, cy - 1), (cx, cy + 1)):
                    if nx < 0 or nx >= w or ny < 0 or ny >= h:
                        continue
                    ni = idx(nx, ny)
                    if visited[ni]:
                        continue
                    visited[ni] = 1
                    rr, gg, bb = pix[nx, ny]
                    if rr >= threshold and gg >= threshold and bb >= threshold:
                        q.append((nx, ny))

            box = Box(x0, y0, x1, y1)
            if count >= min_area:
                boxes.append(box)
    return boxes


def group_rows(boxes: List[Box], y_tol: int) -> List[List[Box]]:
    rows: List[List[Box]] = []
    for b in sorted(boxes, key=lambda k: k.cy):
        placed = False
        for row in rows:
            row_y = sum(it.cy for it in row) / len(row)
            if abs(b.cy - row_y) <= y_tol:
                row.append(b)
                placed = True
                break
        if not placed:
            rows.append([b])
    for row in rows:
        row.sort(key=lambda k: k.cx)
    rows.sort(key=lambda row: sum(it.cy for it in row) / len(row))
    return rows


def build_names() -> List[str]:
    names: List[str] = []
    for suit in SUITS:
        for rank in RANKS:
            names.append(f"{rank}{suit}.png")
    names.append("BACK_1.png")
    names.append("BACK_2.png")
    return names


def flatten_rows(rows: List[List[Box]]) -> List[Box]:
    flat: List[Box] = []
    for row in rows:
        flat.extend(row)
    return flat


def median(values: List[int]) -> int:
    vals = sorted(values)
    n = len(vals)
    if n == 0:
        return 0
    mid = n // 2
    if n % 2 == 1:
        return vals[mid]
    return int((vals[mid - 1] + vals[mid]) / 2)


def main() -> None:
    ap = argparse.ArgumentParser(description="Slice card atlas into per-card files.")
    ap.add_argument("--input", required=True, help="Input atlas PNG with transparent background.")
    ap.add_argument("--out-dir", required=True, help="Output directory for per-card PNG files.")
    ap.add_argument("--alpha-threshold", type=int, default=12, help="Alpha threshold to detect foreground.")
    ap.add_argument("--white-threshold", type=int, default=236, help="RGB threshold for white face detection.")
    ap.add_argument("--min-area", type=int, default=2500, help="Minimum component area to keep.")
    ap.add_argument("--row-tolerance", type=int, default=80, help="Y-center tolerance for row grouping.")
    ap.add_argument("--pad", type=int, default=2, help="Extra transparent padding around each crop.")
    args = ap.parse_args()

    src = Path(args.input)
    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    img = Image.open(src).convert("RGBA")
    alpha = img.getchannel("A")
    comps = find_alpha_components(alpha, threshold=args.alpha_threshold, min_area=args.min_area)

    # Alpha may be noisy after JPEG->alpha conversion. Fallback to white-face detection.
    if len(comps) < 54:
        rgb = img.convert("RGB")
        faces = find_white_face_components(rgb, threshold=args.white_threshold, min_area=max(4000, args.min_area))
        # Card faces are portrait-ish components; keep reasonable aspect range.
        faces = [b for b in faces if 0.25 <= (b.w / max(1, b.h)) <= 0.65 and b.h >= 40 and b.w >= 20]
        faces.sort(key=lambda b: b.area, reverse=True)
        if len(faces) < 54:
            raise SystemExit(
                f"not enough card-like components found: alpha={len(comps)} white_faces={len(faces)} (expected >=54)"
            )
        # Use white-face boxes and expand later to include border.
        main = faces[:80]
        # row tolerance based on observed card height
        tol = max(args.row_tolerance, int(median([b.h for b in main]) * 0.7))
        rows = group_rows(main, y_tol=tol)
    else:
        comps.sort(key=lambda b: b.area, reverse=True)
        main = comps[:54]
        rows = group_rows(main, y_tol=args.row_tolerance)

    flat = flatten_rows(rows)
    flat = flat[:54]
    names = build_names()

    # Normalize output card size for consistent rendering.
    target_w = median([b.w for b in flat])
    target_h = median([b.h for b in flat])
    if target_w < 32 or target_h < 48:
        raise SystemExit(f"detected card size too small: {target_w}x{target_h}")

    w, h = img.size
    for box, name in zip(flat, names):
        # Expand around detected card area to include border/shadow.
        expand_x = max(args.pad, int(box.w * 0.10))
        expand_y = max(args.pad, int(box.h * 0.10))
        x0 = max(0, box.x0 - expand_x)
        y0 = max(0, box.y0 - expand_y)
        x1 = min(w - 1, box.x1 + expand_x)
        y1 = min(h - 1, box.y1 + expand_y)
        crop = img.crop((x0, y0, x1 + 1, y1 + 1))
        canvas = Image.new("RGBA", (target_w, target_h), (0, 0, 0, 0))
        px = (target_w - crop.size[0]) // 2
        py = (target_h - crop.size[1]) // 2
        canvas.alpha_composite(crop, (px, py))
        canvas.save(out_dir / name, format="PNG")

    print(f"[ok] sliced {len(names)} cards -> {out_dir} size={target_w}x{target_h}")


if __name__ == "__main__":
    main()
