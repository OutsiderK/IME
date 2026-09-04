"""Build the compact Chinese/English Windows status icons from their masters."""

from pathlib import Path

from PIL import Image


PROJECT = Path(__file__).resolve().parents[1]
ROOT = PROJECT / "input_methods" / "rime" / "icons"
SOURCES = PROJECT / "assets" / "state-icons"
BLUE = (0x17, 0x46, 0x5F)
ORANGE = (0xC2, 0x55, 0x2C)
SIZES = (16, 20, 24, 32, 48, 64, 128, 256)


def flatten_and_frame(source: Path) -> Image.Image:
    image = Image.open(source).convert("RGBA")
    alpha_box = image.getchannel("A").getbbox()
    if alpha_box is None:
        raise ValueError(f"empty icon master: {source}")
    image = image.crop(alpha_box)

    pixels = image.load()
    for y in range(image.height):
        for x in range(image.width):
            red, green, blue, alpha = pixels[x, y]
            if alpha == 0:
                continue
            color = ORANGE if red > 120 and red > green * 1.45 else BLUE
            pixels[x, y] = (*color, alpha)

    side = max(image.size)
    padding = max(1, round(side * 0.12))
    canvas = Image.new("RGBA", (side + padding * 2, side + padding * 2))
    canvas.alpha_composite(
        image,
        ((canvas.width - image.width) // 2, (canvas.height - image.height) // 2),
    )
    return canvas.resize((256, 256), Image.Resampling.LANCZOS)


def save_family(language: str, variants: list[str]) -> None:
    master = flatten_and_frame(SOURCES / f"{language}-state-master.png")
    master.save(SOURCES / f"{language}-state.png")
    for name in variants:
        master.save(ROOT / name, format="ICO", sizes=[(size, size) for size in SIZES])


save_family(
    "chi",
    [
        "chi.ico",
        "chi_full_capsoff.ico",
        "chi_full_capson.ico",
        "chi_half_capsoff.ico",
        "chi_half_capson.ico",
    ],
)
save_family(
    "eng",
    [
        "eng.ico",
        "eng_full_capsoff.ico",
        "eng_full_capson.ico",
        "eng_half_capsoff.ico",
        "eng_half_capson.ico",
    ],
)
