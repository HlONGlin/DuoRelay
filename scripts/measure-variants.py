#!/usr/bin/env python3
"""Check six full/lite archives and print measured bytes (no extraction)."""
import argparse
import json
from pathlib import Path
import struct
import tarfile
import zipfile


def measure(path, platform, arch, lite):
    if path.suffix == ".zip":
        with zipfile.ZipFile(path) as archive:
            files = {entry.filename: entry.file_size for entry in archive.infolist() if not entry.is_dir()}
            read = archive.read
            return check(path, platform, arch, lite, files, read)
    with tarfile.open(path) as archive:
        files = {entry.name: entry.size for entry in archive.getmembers() if entry.isfile()}
        return check(path, platform, arch, lite, files, lambda name: archive.extractfile(name).read())


def check(path, platform, arch, lite, files, read):
    extension = ".exe" if platform == "windows" else ""
    by_name = {Path(name).name: name for name in files}
    assert "mihomo" + extension in by_name, (path, "missing mihomo")
    if lite:
        assert not any(Path(name).name.lower() in ("sing-box", "sing-box.exe", "libcronet.dll", "libcronet.so") for name in files), (path, "full kernel in lite")
    else:
        assert "sing-box" + extension in by_name, (path, "full kernel missing")
        if platform == "windows":
            assert "libcronet.dll" in by_name, (path, "full Cronet missing")
    assert not any(Path(name).suffix.lower() in (".dat", ".mmdb", ".metadb") for name in files), (path, "Geo data bundled")
    binaries = ["duorelay" + extension, "mihomo" + extension] + ([] if lite else ["sing-box" + extension])
    for name in binaries:
        data = read(by_name[name])
        if platform == "windows":
            assert data[:2] == b"MZ"
            offset = struct.unpack_from("<I", data, 60)[0]
            assert data[offset:offset + 4] == b"PE\0\0" and struct.unpack_from("<H", data, offset + 4)[0] == 0x8664, (path, name, "wrong Windows architecture")
        else:
            assert data[:4] == b"\x7fELF" and struct.unpack_from("<H", data, 18)[0] == {"amd64": 62, "arm64": 183}[arch], (path, name, "wrong ELF architecture")
    panel = files[by_name["duorelay" + extension]]
    mihomo = files[by_name["mihomo" + extension]]
    singbox = files.get(by_name.get("sing-box" + extension, ""), 0)
    cronet = files.get(by_name.get("libcronet.dll", ""), 0)
    info = json.loads(read(by_name["build-info.json"]).decode("utf-8-sig"))
    assert info["platform"] == platform + "/" + arch
    assert info["variant"] == ("lite" if lite else "full")
    return {"archive": path.name, "platform": platform + "/" + arch, "variant": info["variant"], "version": info["version"], "compressed_bytes": path.stat().st_size,
            "panel_bytes": panel, "mihomo_bytes": mihomo, "singbox_bytes": singbox, "cronet_bytes": cronet, "geo_bytes": 0,
            "other_bytes": sum(files.values()) - panel - mihomo - singbox - cronet, "files": files}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dist", type=Path, default=Path("dist"))
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    rows = []
    for platform, arch in [("linux", "amd64"), ("linux", "arm64"), ("windows", "amd64")]:
        for lite in [False, True]:
            name = f"duorelay-lite-{args.version}-{arch}" if lite else f"duorelay-{platform}-{arch}-{args.version}"
            name += ".zip" if platform == "windows" else ".tar.gz"
            rows.append(measure(args.dist / name, platform, arch, lite))
    assert len({row["version"] for row in rows}) == 1
    print(json.dumps(rows, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
