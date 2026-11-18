#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
HTML Bundler for QuickNetScan
Embeds CSS, JS, and Images into a single self-contained HTML file.
"""

import base64
import re
from pathlib import Path

ROOT_DIR = Path(__file__).resolve().parent.parent
STATIC_DIR = ROOT_DIR / "web" / "static"

def bundle(output_dir):
    out_path = Path(output_dir) / "index_embed.html"
    index_html = STATIC_DIR / "index.html"

    with open(index_html, "r", encoding="utf-8") as f:
        html = f.read()

    # Inline CSS (use lambda to prevent re.sub from interpreting backslashes)
    css_path = STATIC_DIR / "css" / "style.css"
    if css_path.exists():
        with open(css_path, "r", encoding="utf-8") as f:
            css_content = f.read()
        html = re.sub(
            r'<link\s+rel="stylesheet"\s+href="css/style\.css"[^>]*>',
            lambda _: f"<style>\n{css_content}\n</style>",
            html
        )

    # Inline image assets
    assets_dir = STATIC_DIR / "assets"
    jpg_badge = assets_dir / "maktak105-V04-01.jpg"
    if jpg_badge.exists():
        with open(jpg_badge, "rb") as f:
            b64_badge = base64.b64encode(f.read()).decode("ascii")
        html = html.replace('src="assets/maktak105-V04-01.jpg"', f'src="data:image/jpeg;base64,{b64_badge}"')

    svg_logo = assets_dir / "QuickNetScan-icon.svg"
    if svg_logo.exists():
        with open(svg_logo, "rb") as f:
            b64_logo = base64.b64encode(f.read()).decode("ascii")
        html = html.replace('src="assets/QuickNetScan-icon.svg"', f'src="data:image/svg+xml;base64,{b64_logo}"')

    # Inline JS scripts in order
    js_files = ["wasm_exec.js", "app.js"]
    scripts_content = []
    for js_name in js_files:
        js_path = STATIC_DIR / "js" / js_name
        if js_path.exists():
            with open(js_path, "r", encoding="utf-8") as f:
                scripts_content.append(f.read())

    combined_js = "\n\n".join(scripts_content)
    # Remove script tags and replace with combined script
    html = re.sub(r'<script\s+src="js/[^"]+"\s*></script>', "", html)
    html = html.replace("</body>", f"<script>\n{combined_js}\n</script>\n</body>")

    with open(out_path, "w", encoding="utf-8") as f:
        f.write(html)

    print(f"[Bundle] Created self-contained HTML: {out_path} ({len(html)/1024:.1f} KB)")
    return str(out_path)

if __name__ == "__main__":
    bundle(ROOT_DIR / "build" / "intermediate")
