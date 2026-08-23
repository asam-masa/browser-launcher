#!/usr/bin/env python3
"""Check whether local link targets in Markdown files exist."""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from urllib.parse import unquote, urlsplit


INLINE_LINK = re.compile(
    r"!?\[[^\]]*\]\((?P<target><[^>]+>|[^\s)]+)(?:\s+[^)]*)?\)"
)
REFERENCE_LINK = re.compile(r"^\s*\[[^\]]+\]:\s*(?P<target><[^>]+>|\S+)")
FENCE = re.compile(r"^\s*(?P<marker>`{3,}|~{3,})")
EXCLUDED_DIRECTORIES = {".git", "node_modules", "vendor", "dist", "build"}


@dataclass(frozen=True)
class Finding:
    source: Path
    line: int
    target: str
    reason: str


def extract_targets(markdown: str) -> list[tuple[int, str]]:
    """Return local and external raw link targets outside fenced code blocks."""
    targets: list[tuple[int, str]] = []
    fence_marker: str | None = None

    for line_number, line in enumerate(markdown.splitlines(), start=1):
        fence = FENCE.match(line)
        if fence:
            marker = fence.group("marker")[0]
            if fence_marker is None:
                fence_marker = marker
            elif fence_marker == marker:
                fence_marker = None
            continue

        if fence_marker is not None:
            continue

        targets.extend(
            (line_number, match.group("target"))
            for match in INLINE_LINK.finditer(line)
        )

        reference = REFERENCE_LINK.match(line)
        if reference:
            targets.append((line_number, reference.group("target")))

    return targets


def normalize_local_target(raw_target: str) -> str | None:
    """Return a decoded local path, or None for links not checked here."""
    target = raw_target.strip()
    if target.startswith("<") and target.endswith(">"):
        target = target[1:-1].strip()

    if not target or target.startswith("#") or target.startswith("//"):
        return None

    parsed = urlsplit(target)
    if parsed.scheme or parsed.netloc:
        return None

    path = unquote(parsed.path)
    return path or None


def resolve_target(root: Path, source: Path, target: str) -> tuple[Path, str | None]:
    """Resolve a repository-local target and explain invalid escapes."""
    if target.startswith("/"):
        candidate = root / target.lstrip("/")
    else:
        candidate = source.parent / target

    resolved = candidate.resolve(strict=False)
    try:
        resolved.relative_to(root)
    except ValueError:
        return resolved, "target resolves outside repository"
    return resolved, None


def check_text(root: Path, source: Path, markdown: str) -> list[Finding]:
    """Check Markdown text and return missing or escaping local targets."""
    findings: list[Finding] = []
    for line, raw_target in extract_targets(markdown):
        target = normalize_local_target(raw_target)
        if target is None:
            continue

        resolved, reason = resolve_target(root, source, target)
        if reason is not None:
            findings.append(Finding(source, line, raw_target, reason))
        elif not resolved.exists():
            findings.append(Finding(source, line, raw_target, "target does not exist"))

    return findings


def iter_markdown_files(root: Path, paths: list[Path]) -> list[Path]:
    """Collect Markdown files without traversing generated dependency trees."""
    files: set[Path] = set()
    candidates = paths or [root]

    for candidate in candidates:
        resolved = candidate.resolve(strict=False)
        if resolved.is_file() and resolved.suffix.lower() == ".md":
            files.add(resolved)
            continue
        if not resolved.is_dir():
            continue

        for path in resolved.rglob("*.md"):
            relative_parts = path.relative_to(root).parts
            if not EXCLUDED_DIRECTORIES.intersection(relative_parts):
                files.add(path.resolve())

    return sorted(files)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Check local Markdown link targets without modifying files."
    )
    parser.add_argument("root", type=Path, help="repository root")
    parser.add_argument(
        "paths",
        nargs="*",
        type=Path,
        help="optional Markdown files or directories; defaults to the repository root",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    root = args.root.resolve(strict=False)
    if not root.is_dir():
        print(f"error: repository root is not a directory: {root}", file=sys.stderr)
        return 2

    paths = [
        (path if path.is_absolute() else root / path).resolve(strict=False)
        for path in args.paths
    ]
    for path in paths:
        try:
            path.relative_to(root)
        except ValueError:
            print(f"error: inspection path is outside repository: {path}", file=sys.stderr)
            return 2
        if not path.exists():
            print(f"error: inspection path does not exist: {path}", file=sys.stderr)
            return 2

    files = iter_markdown_files(root, paths)
    findings: list[Finding] = []

    try:
        for source in files:
            markdown = source.read_text(encoding="utf-8")
            findings.extend(check_text(root, source, markdown))
    except (OSError, UnicodeError) as error:
        print(f"error: could not read Markdown: {error}", file=sys.stderr)
        return 2

    for finding in findings:
        source = finding.source.relative_to(root)
        print(f"{source}:{finding.line}: {finding.reason}: {finding.target}")

    if findings:
        print(f"Found {len(findings)} invalid local link target(s) in {len(files)} file(s).")
        return 1

    print(f"Checked {len(files)} Markdown file(s); no invalid local link targets found.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
