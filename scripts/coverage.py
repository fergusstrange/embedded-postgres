"""Merge native/unit/subprocess Go profiles and enforce the production floor.

Counts each statement block once, covered when any native run executed it.
OS-specific files remain in the denominator. Only compiled documentation examples
are outside production scope; no production package or line is excluded.
"""
import argparse
import pathlib
import sys


def merge(paths):
    blocks = {}
    for path in paths:
        for line in path.read_text().splitlines()[1:]:
            location, statements, count = line.split()
            if "/examples/" in location or "/examples:" in location:
                continue
            key = (location, int(statements))
            blocks[key] = max(blocks.get(key, 0), int(count))
    return blocks


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("paths", nargs="+")
    parser.add_argument("--minimum", type=float, default=90.0)
    parser.add_argument("--output", default="coverage.out")
    args = parser.parse_args()
    paths = []
    for name in args.paths:
        path = pathlib.Path(name)
        paths.extend(sorted(path.rglob("*.out")) if path.is_dir() else [path])
    paths = [p for p in paths if p.resolve() != pathlib.Path(args.output).resolve()]
    blocks = merge(paths)
    if not blocks:
        raise SystemExit("No coverage blocks found")
    total = sum(n for _, n in blocks)
    covered = sum(n for (_, n), count in blocks.items() if count)
    percent = 100 * covered / total
    with open(args.output, "w") as output:
        output.write("mode: set\n")
        for (location, n), count in sorted(blocks.items()):
            output.write(f"{location} {n} {int(count > 0)}\n")
    files = {}
    for (location, n), count in blocks.items():
        name = location.split(":")[0]
        hit, size = files.get(name, (0, 0))
        files[name] = (hit + n * (count > 0), size + n)
    for name, (hit, size) in sorted(files.items()):
        print(f"{(100 * hit / size) if size else 100:6.2f}% {name}")
    print(f"Production coverage: {percent:.2f}% ({covered}/{total} statements); required {args.minimum:.2f}%")
    return int(percent < args.minimum)


if __name__ == "__main__":
    sys.exit(main())
