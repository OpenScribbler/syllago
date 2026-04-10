"""
POC Document Assembly Runner — Claude Web Helper

Reads a recipe YAML and pre-fetched content from a workspace directory,
builds the input dicts, and calls assemble_web(). Deterministic — no
content generation, just file loading and assembly.

Expected workspace layout:
    /home/claude/workspace/
        recipe.yaml                          <- the recipe to assemble
        content/shared/cover.md              <- content modules
        content/shared/introduction.md
        content/infrastructure/agent_controller.md
        content/client_identity/ec2_iam_role.md
        ...
        css/aembit.css                       <- stylesheet
        customer_logo.b64                    <- optional, base64 text file

Assets (aembit-logo-white.png.b64, aembit-icon-small.png.b64) are bundled
in the skill and should be copied to the workspace by Claude before running.

Usage:
    python run_assembly.py [--workspace /home/claude/workspace] [--pdf] [--html]

Output:
    --pdf (default): writes output.pdf to the workspace
    --html: writes output.html to the workspace instead
"""

import argparse
import sys
from pathlib import Path

# assembler_web.py must be in the same directory or already on sys.path
from assembler_web import assemble_web


def load_content_modules(content_dir: Path) -> dict[str, str]:
    """Walk the content directory and build the content_modules dict.

    Maps relative paths (without .md extension) to file contents.
    e.g. content/shared/cover.md -> {"shared/cover": "..."}
    """
    modules = {}
    for md_file in content_dir.rglob("*.md"):
        # Relative path from content_dir, without .md extension
        key = str(md_file.relative_to(content_dir).with_suffix(""))
        modules[key] = md_file.read_text()
    return modules


def load_assets(workspace: Path) -> dict[str, str]:
    """Load base64-encoded asset files from the workspace.

    Looks for *.b64 files in the assets/ directory.
    Maps the original filename (without .b64) to the base64 content.
    e.g. assets/aembit-logo-white.png.b64 -> {"aembit-logo-white.png": "..."}
    """
    assets = {}
    assets_dir = workspace / "assets"
    if assets_dir.exists():
        for b64_file in assets_dir.glob("*.b64"):
            # Remove .b64 suffix to get original filename
            original_name = b64_file.stem  # e.g. "aembit-logo-white.png"
            assets[original_name] = b64_file.read_text().strip()
    return assets


def main():
    parser = argparse.ArgumentParser(description="POC Document Assembly Runner")
    parser.add_argument(
        "--workspace",
        default="/home/claude/workspace",
        help="Workspace directory containing recipe, content, css, assets",
    )
    parser.add_argument(
        "--pdf",
        action="store_true",
        default=True,
        help="Render PDF output (default)",
    )
    parser.add_argument(
        "--html",
        action="store_true",
        help="Render HTML output instead of PDF",
    )
    args = parser.parse_args()

    workspace = Path(args.workspace)
    render_pdf = not args.html

    # Load recipe
    recipe_path = workspace / "recipe.yaml"
    if not recipe_path.exists():
        print(f"ERROR: Recipe not found at {recipe_path}", file=sys.stderr)
        sys.exit(1)

    import yaml
    recipe = yaml.safe_load(recipe_path.read_text())

    # Load content modules
    content_dir = workspace / "content"
    if not content_dir.exists():
        print(f"ERROR: Content directory not found at {content_dir}", file=sys.stderr)
        sys.exit(1)
    content_modules = load_content_modules(content_dir)
    print(f"Loaded {len(content_modules)} content modules")

    # Load CSS
    css_path = workspace / "css" / "aembit.css"
    if not css_path.exists():
        print(f"ERROR: CSS not found at {css_path}", file=sys.stderr)
        sys.exit(1)
    css_text = css_path.read_text()

    # Load assets
    assets = load_assets(workspace)
    print(f"Loaded {len(assets)} assets: {list(assets.keys())}")

    # Load customer logo (optional)
    customer_logo = None
    logo_path = workspace / "customer_logo.b64"
    if logo_path.exists():
        customer_logo = logo_path.read_text().strip()
        print("Loaded customer logo")

    # Run assembly
    result = assemble_web(
        recipe=recipe,
        content_modules=content_modules,
        css_text=css_text,
        assets=assets,
        customer_logo=customer_logo,
        render_pdf=render_pdf,
    )

    # Write output
    if render_pdf:
        output_path = workspace / "output.pdf"
        output_path.write_bytes(result)
        print(f"PDF written to {output_path} ({len(result)} bytes)")
    else:
        output_path = workspace / "output.html"
        output_path.write_text(result)
        print(f"HTML written to {output_path} ({len(result)} chars)")


if __name__ == "__main__":
    main()
