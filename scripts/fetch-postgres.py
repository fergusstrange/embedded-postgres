"""CI-only download helper; production downloads are implemented in Go."""
import hashlib
import json
import pathlib
import sys
import urllib.request

version, target, output = sys.argv[1:]
name = f"postgresql-{version}-{target}.tar.gz"
manifest = json.loads((pathlib.Path(__file__).parent.parent / "internal/binaries/manifest.json").read_text())
entry = manifest[name]
path = pathlib.Path(output)
path.parent.mkdir(parents=True, exist_ok=True)
with urllib.request.urlopen(entry["url"], timeout=120) as source, path.open("wb") as dest:
    while chunk := source.read(1024 * 1024):
        dest.write(chunk)
if hashlib.sha256(path.read_bytes()).hexdigest() != entry["sha256"]:
    path.unlink()
    raise SystemExit("PostgreSQL archive checksum mismatch")
