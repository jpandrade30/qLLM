#!/usr/bin/env python3
"""Render root Markdown into site/*.html with the product chrome (stdlib only).

Usage:
  python scripts/dev/render_site_md.py
  python scripts/dev/render_site_md.py --only changelog

Currently ships:
  CHANGELOG.md → site/changelog.html
"""

from __future__ import annotations

import argparse
import html
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SITE = ROOT / "site"

# Journey: start → configure → connect → ask → trust → deepen → reference → history → community
NAV_LINKS = [
    ("index.html", "Home"),
    ("get-started.html", "Get started"),
    ("configure.html", "Configure"),
    ("connectors.html", "Connectors"),
    ("query.html", "Query"),
    ("security.html", "Security"),
    ("docs.html", "Docs"),
    ("protocol.html", "Protocol"),
    ("decisions.html", "Decisions"),
    ("changelog.html", "Changelog"),
    ("contribute.html", "Contribute"),
]


def nav_html(current: str) -> str:
    parts = ['        <nav id="site-nav" class="site-nav" aria-label="Primary">']
    for href, label in NAV_LINKS:
        cur = ' aria-current="page"' if href == current else ""
        parts.append(f'          <a href="{href}"{cur}>{label}</a>')
    parts.append("        </nav>")
    return "\n".join(parts)


def inline_md(text: str) -> str:
    """Escape then apply a small inline Markdown subset."""
    s = html.escape(text)

    def link_repl(m: re.Match[str]) -> str:
        label, url = m.group(1), m.group(2)
        if url.startswith(("http://", "https://", "mailto:")):
            href = html.escape(url, quote=True)
        elif url.startswith("/"):
            href = html.escape("https://github.com/jpandrade30/qLLM" + url, quote=True)
        else:
            # Repo-relative path from CHANGELOG.md at repo root
            href = html.escape(
                "https://github.com/jpandrade30/qLLM/blob/main/" + url.lstrip("./"),
                quote=True,
            )
        return f'<a href="{href}">{label}</a>'

    s = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", link_repl, s)
    s = re.sub(r"`([^`]+)`", r"<code>\1</code>", s)
    s = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", s)
    s = re.sub(r"(?<!\*)\*([^*]+)\*(?!\*)", r"<em>\1</em>", s)
    return s


def md_to_html(md: str) -> str:
    lines = md.replace("\r\n", "\n").split("\n")
    out: list[str] = []
    i = 0
    in_ul = False
    in_p = False

    def close_ul() -> None:
        nonlocal in_ul
        if in_ul:
            out.append("</ul>")
            in_ul = False

    def close_p() -> None:
        nonlocal in_p
        if in_p:
            out.append("</p>")
            in_p = False

    while i < len(lines):
        line = lines[i]
        stripped = line.strip()

        if not stripped:
            close_p()
            close_ul()
            i += 1
            continue

        if stripped.startswith("#"):
            close_p()
            close_ul()
            level = len(stripped) - len(stripped.lstrip("#"))
            level = min(max(level, 1), 4)
            title = stripped[level:].strip()
            # Skip the top H1 — page chrome already has <h1>
            if level == 1 and not out:
                i += 1
                continue
            slug = re.sub(r"[^a-z0-9]+", "-", title.lower()).strip("-")
            # Keep a version anchors like unreleased / 0-3-2
            out.append(f'<h{level} id="{html.escape(slug, quote=True)}">{inline_md(title)}</h{level}>')
            i += 1
            continue

        if stripped.startswith(("- ", "* ")):
            close_p()
            if not in_ul:
                out.append("<ul>")
                in_ul = True
            out.append(f"<li>{inline_md(stripped[2:].strip())}</li>")
            i += 1
            continue

        close_ul()
        if not in_p:
            out.append("<p>")
            in_p = True
        else:
            out.append("<br />")
        out.append(inline_md(stripped))
        i += 1

    close_p()
    close_ul()
    return "\n".join(out)


def page_shell(
    *,
    title: str,
    description: str,
    current: str,
    mono_label: str,
    heading: str,
    lead: str,
    body_html: str,
) -> str:
    return f"""<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{html.escape(title)}</title>
    <meta name="description" content="{html.escape(description)}" />
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link
      href="https://fonts.googleapis.com/css2?family=IBM+Plex+Sans:wght@400;500;600&family=JetBrains+Mono:wght@400;500&family=Space+Grotesk:wght@500;600&display=swap"
      rel="stylesheet"
    />
    <link rel="icon" href="assets/StudyingCat.ico" type="image/x-icon" />
    <link rel="stylesheet" href="assets/styles.css" />
  </head>
  <body>
    <header class="site-header">
      <div class="site-header__inner">
        <a class="brand" href="index.html">q<span>LLM</span></a>
        <button class="nav-toggle" type="button" aria-expanded="false" aria-controls="site-nav">
          Menu
        </button>
{nav_html(current)}
      </div>
    </header>

    <main class="wrap">
      <header class="page-head">
        <span class="mono-label">{html.escape(mono_label)}</span>
        <h1>{html.escape(heading)}</h1>
        <p class="lead">{lead}</p>
      </header>

      <section class="md-body reveal">
{body_html}
      </section>

      <div class="cta-band reveal">
        <div>
          <h2>Protocol bump notes</h2>
          <p style="margin: 0.5rem 0 0">
            See what changed between protocol <code>0.1.0</code> and <code>0.2.0</code>.
          </p>
        </div>
        <a class="btn btn--ghost" href="protocol.html">Protocol →</a>
      </div>
    </main>

    <footer class="site-footer">
      <span>qLLM product site</span>
      <span>
        <a href="https://github.com/jpandrade30/qLLM">GitHub</a>
        ·
        <a href="https://github.com/jpandrade30/qLLM/blob/main/CHANGELOG.md">CHANGELOG.md</a>
        ·
        <a href="https://github.com/jpandrade30/qLLM/tree/main/planning">planning/</a>
      </span>
    </footer>
    <script src="assets/site.js"></script>
  </body>
</html>
"""


def render_changelog() -> Path:
    src = ROOT / "CHANGELOG.md"
    md = src.read_text(encoding="utf-8")
    body = md_to_html(md)
    lead = (
        'Generated from root <a href="https://github.com/jpandrade30/qLLM/blob/main/CHANGELOG.md">'
        "CHANGELOG.md</a> (Keep a Changelog). "
        "Protocol versions are the <code>protocolVersion</code> field; product releases are separate."
    )
    page = page_shell(
        title="Changelog — qLLM",
        description="qLLM changelog: protocol and product releases, Keep a Changelog format.",
        current="changelog.html",
        mono_label="History",
        heading="Changelog",
        lead=lead,
        body_html=body,
    )
    out = SITE / "changelog.html"
    out.write_text(page, encoding="utf-8", newline="\n")
    return out


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--only", choices=("changelog",), default=None)
    args = parser.parse_args()
    written: list[Path] = []
    if args.only in (None, "changelog"):
        written.append(render_changelog())
    for path in written:
        print(f"wrote {path} ({path.stat().st_size} bytes)")


if __name__ == "__main__":
    main()
