#!/usr/bin/env python3
"""Generate docs/zenzx-model-catalog.pdf.

Builds the headless binary, boots every supported model long enough to
clear its boot sequence, captures one screenshot per model, and
assembles them into a labelled PDF catalog. Everything -- the binary
and the intermediate screenshots -- lives in a temp directory that's
cleaned up automatically; only the finished PDF lands in docs/.

Usage (from the repository root):
    python3 docs/generate-model-catalog.py

Requires: a Go toolchain on PATH, and reportlab (pip install reportlab).
"""
import os
import subprocess
import sys
import tempfile
from pathlib import Path

from reportlab.lib import colors
from reportlab.lib.enums import TA_CENTER
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import inch
from reportlab.platypus import (
    Image,
    Paragraph,
    SimpleDocTemplate,
    Spacer,
    Table,
    TableStyle,
)

REPO_ROOT = Path(__file__).resolve().parent.parent
ROM_DIR = REPO_ROOT / "rom"
OUT_PDF = REPO_ROOT / "docs" / "zenzx-model-catalog.pdf"

# 200 frames matches smoke_headless.sh's own proven frame count -- enough
# for every model's boot sequence, including TS2068's different NTSC
# timing, to clear and paint its copyright/menu screen. At 100 frames
# TS2068 in particular is still mid-boot.
FRAMES = 200

# (model flag, display name shown under each screenshot)
MODELS = [
    ("48k", "ZX Spectrum 16K/48K"),
    ("128k", "Sinclair 128K \u201cToastrack\u201d"),
    ("plus2", "ZX Spectrum +2 (grey)"),
    ("plus2a", "ZX Spectrum +2A/+2B"),
    ("plus3", "ZX Spectrum +3/+3B"),
    ("spanish48k", "Spanish 48K"),
    ("spanish128k", "Spanish 128K"),
    ("spanishplus2", "Spanish Spectrum +2"),
    ("spanishplus3", "Spanish Spectrum +3"),
    ("ts2068", "Timex Sinclair 2068"),
]


def build_binary(work_dir: Path) -> Path:
    binary = work_dir / "zenzx-headless"
    print("Building zenzx-headless...")
    result = subprocess.run(
        ["go", "build", "-tags", "headless", "-o", str(binary), "."],
        cwd=REPO_ROOT,
        env={**os.environ, "CGO_ENABLED": "0"},
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        sys.exit(f"Build failed:\n{result.stderr}")
    return binary


def capture_screenshots(binary: Path, shot_dir: Path) -> dict:
    """Boot each model and capture one screenshot each. Returns
    {model_flag: screenshot_path}, skipping (with a warning, not a
    hard failure) any model whose boot doesn't produce one -- one bad
    model shouldn't block a catalog of the other nine."""
    paths = {}
    for flag, _ in MODELS:
        result = subprocess.run(
            [
                str(binary),
                f"-model={flag}",
                f"-frames={FRAMES}",
                "-quiet",
                f"-shot-dir={shot_dir}",
                f"-shot-prefix={flag}",
                f"-romdir={ROM_DIR}",
            ],
            capture_output=True,
            text=True,
        )
        expected = shot_dir / f"{flag}-frame{FRAMES:06d}.png"
        if result.returncode == 0 and expected.exists():
            paths[flag] = expected
        else:
            print(
                f"Warning: {flag} did not produce a screenshot, skipping it "
                f"in the catalog -- {result.stderr.strip()}"
            )
    return paths


def build_pdf(shots: dict):
    styles = getSampleStyleSheet()
    title_style = ParagraphStyle(
        "CatTitle", parent=styles["Title"], fontSize=20, spaceAfter=4
    )
    sub_style = ParagraphStyle(
        "CatSub",
        parent=styles["Normal"],
        fontSize=10,
        textColor=colors.HexColor("#555555"),
        spaceAfter=18,
    )
    cell_label_style = ParagraphStyle(
        "CellLabel",
        parent=styles["Normal"],
        fontSize=9,
        alignment=TA_CENTER,
        fontName="Helvetica-Bold",
    )
    cell_flag_style = ParagraphStyle(
        "CellFlag",
        parent=styles["Normal"],
        fontSize=7.5,
        alignment=TA_CENTER,
        fontName="Courier",
        textColor=colors.HexColor("#666666"),
    )

    doc = SimpleDocTemplate(
        str(OUT_PDF),
        pagesize=letter,
        topMargin=0.7 * inch,
        bottomMargin=0.7 * inch,
        leftMargin=0.7 * inch,
        rightMargin=0.7 * inch,
    )

    story = [
        Paragraph("zenzx model catalog", title_style),
        Paragraph(
            f"Boot screens for every supported model, captured at {FRAMES} "
            "emulated frames (headless build). Each screenshot is genuine "
            "emulator output from the standard ROM set, not a mockup. "
            "Regenerate with <font face=\"Courier\">python3 "
            "docs/generate-model-catalog.py</font> from the repository root.",
            sub_style,
        ),
    ]

    # 256x192 is the real Spectrum screen's native resolution (4:3) --
    # scaled up here for legibility, aspect ratio preserved exactly.
    IMG_W, IMG_H = 2.6 * inch, 1.95 * inch
    cell_data, row = [], []
    for flag, name in MODELS:
        if flag not in shots:
            continue
        img = Image(str(shots[flag]), width=IMG_W, height=IMG_H)
        img.hAlign = "CENTER"
        row.append(
            [
                img,
                Spacer(1, 3),
                Paragraph(name, cell_label_style),
                Paragraph(f"-model={flag}", cell_flag_style),
            ]
        )
        if len(row) == 2:
            cell_data.append(row)
            row = []
    if row:
        row.append([Spacer(1, 1)])
        cell_data.append(row)

    if not cell_data:
        sys.exit("No screenshots to lay out -- nothing to build a PDF from.")

    t = Table(cell_data, colWidths=[3.1 * inch, 3.1 * inch])
    t.setStyle(
        TableStyle(
            [
                ("VALIGN", (0, 0), (-1, -1), "TOP"),
                ("ALIGN", (0, 0), (-1, -1), "CENTER"),
                ("TOPPADDING", (0, 0), (-1, -1), 10),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 14),
                ("LINEBELOW", (0, 0), (-1, -2), 0.5, colors.HexColor("#dddddd")),
            ]
        )
    )
    story.append(t)
    doc.build(story)


def main():
    with tempfile.TemporaryDirectory(prefix="zenzx-catalog-") as tmp:
        work_dir = Path(tmp)
        binary = build_binary(work_dir)
        shots = capture_screenshots(binary, work_dir)
        if not shots:
            sys.exit("No screenshots captured -- nothing to build a catalog from.")
        build_pdf(shots)
    print(f"Catalog written: {OUT_PDF}")


if __name__ == "__main__":
    main()
