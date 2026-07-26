#!/usr/bin/env python3
"""Prepare Cutable web assets while preserving the supplied mark pixel-for-pixel.

The source image contains a baked checkerboard. Only bright, near-neutral pixels
connected to the outer edge are removed, which leaves the enclosed white spokes
inside the mark untouched.
"""

from collections import deque
from pathlib import Path

from PIL import Image


ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "assets/brand/cutable-source.png"
SOURCE_V2 = ROOT / "assets/brand/cutable-v2-source.png"
PUBLIC = ROOT / "apps/frontend/public/brand"
APP = ROOT / "apps/frontend/app"


def is_background(pixel: tuple[int, int, int, int]) -> bool:
    red, green, blue, _ = pixel
    return min(red, green, blue) >= 205 and max(red, green, blue) - min(red, green, blue) <= 9


def remove_edge_background(source: Image.Image) -> Image.Image:
    image = source.convert("RGBA")
    width, height = image.size
    pixels = image.load()
    visited = bytearray(width * height)
    queue: deque[tuple[int, int]] = deque()

    for x in range(width):
        queue.append((x, 0))
        queue.append((x, height - 1))
    for y in range(height):
        queue.append((0, y))
        queue.append((width - 1, y))

    while queue:
        x, y = queue.popleft()
        offset = y * width + x
        if visited[offset]:
            continue
        visited[offset] = 1
        if not is_background(pixels[x, y]):
            continue

        red, green, blue, _ = pixels[x, y]
        pixels[x, y] = (red, green, blue, 0)
        for next_y in range(max(0, y - 1), min(height, y + 2)):
            for next_x in range(max(0, x - 1), min(width, x + 2)):
                if not visited[next_y * width + next_x]:
                    queue.append((next_x, next_y))

    return image


def contained_canvas(mark: Image.Image, size: int, padding: int) -> Image.Image:
    alpha = mark.getchannel("A")
    bounds = alpha.getbbox()
    if bounds is None:
        raise RuntimeError("supplied logo has no visible pixels")
    cropped = mark.crop(bounds)
    available = size - 2 * padding
    cropped.thumbnail((available, available), Image.Resampling.LANCZOS)
    canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    canvas.alpha_composite(cropped, ((size - cropped.width) // 2, (size - cropped.height) // 2))
    return canvas


def main() -> None:
    PUBLIC.mkdir(parents=True, exist_ok=True)
    if SOURCE_V2.exists():
        mark = Image.open(SOURCE_V2).convert("RGBA")
    else:
        mark = remove_edge_background(Image.open(SOURCE))

    mark.save(PUBLIC / "cutable-mark.png", optimize=True)
    contained_canvas(mark, 512, 20).save(APP / "icon.png", optimize=True)
    contained_canvas(mark, 180, 8).save(APP / "apple-icon.png", optimize=True)
    contained_canvas(mark, 256, 10).save(
        APP / "favicon.ico",
        sizes=[(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)],
    )


if __name__ == "__main__":
    main()
