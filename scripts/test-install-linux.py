#!/usr/bin/env python3
"""Exercise installer upgrade/data handling without creating real system services."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


def main():
    if os.name != "posix":
        raise SystemExit("Run this test on Linux")
    root = Path(__file__).resolve().parent.parent
    with tempfile.TemporaryDirectory(prefix="duorelay-installer-") as temporary:
        work = Path(temporary)
        package, prefix, data, units, commands = (work / name for name in ("package", "installed", "data", "units", "bin"))
        for directory in (package, prefix, data, units, commands):
            directory.mkdir()
        shutil.copytree(root / "deploy", package / "deploy")
        shutil.copy2(shutil.which("true"), package / "duorelay")
        shutil.copy2(shutil.which("sleep"), prefix / "duorelay")
        (package / "kernels").mkdir()
        shutil.copy2(shutil.which("true"), package / "kernels/mihomo")
        (data / "duorelay.db").write_bytes(b"existing-database")
        (data / "duorelay.db.key").write_bytes(b"key-must-survive")
        (commands / "stub").write_text('''#!/usr/bin/env python3
import os,sys,subprocess
from pathlib import Path
name=Path(sys.argv[0]).name
args=sys.argv[1:]
if name=='id': print('0')
elif name=='install':
    clean=[]
    i=0
    while i<len(args):
        if args[i] in ('-o','-g'): i+=2
        else: clean.append(args[i]); i+=1
    sys.exit(subprocess.call([os.environ['INSTALLER_REAL_INSTALL'],*clean]))
''')
        (commands / "stub").chmod(0o755)
        for name in ("systemctl", "getent", "id", "groupadd", "useradd", "chown", "install"):
            (commands / name).symlink_to(commands / "stub")
        # Redirect only the service destination. All copies and atomic renames
        # still use real Linux filesystem operations against a running ELF.
        text = (root / "scripts/install-linux.sh").read_text().replace("/etc/systemd/system", str(units))
        script = work / "install.sh"
        script.write_text(text)
        env = dict(os.environ, PATH=str(commands) + ":" + os.environ["PATH"], INSTALLER_REAL_INSTALL=shutil.which("install"))
        running = subprocess.Popen([str(prefix / "duorelay"), "60"])
        try:
            for index, transparent in enumerate((False, True, False)):
                args = ["bash", str(script), "--package", str(package), "--prefix", str(prefix), "--data-dir", str(data)]
                if index == 1:
                    args += ["--backup"]
                if transparent:
                    args += ["--transparent"]
                result = subprocess.run(args, env=env, capture_output=True, text=True)
                assert result.returncode == 0, (result.stdout, result.stderr)
                assert (data / "duorelay.db").read_bytes() == b"existing-database"
                assert (data / "duorelay.db.key").read_bytes() == b"key-must-survive"
                assert (prefix / "duorelay").read_bytes() == (package / "duorelay").read_bytes()
                assert (prefix / "kernels/mihomo").read_bytes() == (package / "kernels/mihomo").read_bytes()
                unit = (units / "duorelay.service").read_text()
                assert "StateDirectory=duorelay" in unit and "${STATE_DIRECTORY}" in unit
                assert "StartLimitBurst=3" in unit and "RestartSec=15" in unit
                custom = (units / "duorelay.service.d/custom-data.conf").read_text()
                assert "StateDirectory=\n" in custom and f"Environment=STATE_DIRECTORY={data}" in custom
                dropin = units / "duorelay.service.d/transparent.conf"
                assert dropin.exists() == transparent
                if transparent:
                    assert "User=root" in dropin.read_text() and "CAP_NET_ADMIN" in dropin.read_text()
                if index > 0:
                    assert (units / "duorelay-backup.service").exists()
                    assert (units / "duorelay-backup.timer").exists()
                    expected_user = "root" if transparent else "duorelay"
                    assert f"User={expected_user}" in (units / "duorelay-backup.service").read_text()
                if index > 0 and shutil.which("systemd-analyze"):
                    result = subprocess.run(["systemd-analyze", "verify", str(units / "duorelay.service"), str(units / "duorelay-backup.service"), str(units / "duorelay-backup.timer")], capture_output=True, text=True)
                    assert result.returncode == 0, result.stderr
                assert running.poll() is None, "upgrade terminated the running executable"
            result = subprocess.run(["bash", str(script), "--user-mode", "--transparent"], env=env, capture_output=True)
            assert result.returncode != 0
            result = subprocess.run(["bash", str(script), "--prefix", str(work / "bad path")], env=env, capture_output=True)
            assert result.returncode != 0
        finally:
            running.terminate()
            running.wait(timeout=5)
    print("PASS: installer atomically replaces a running ELF, preserves data/key, renders service modes and rejects invalid options")
    if shutil.which("systemd-analyze"):
        print("PASS: systemd-analyze verifies rendered services, drop-ins and backup timer")
    else:
        print("UNVERIFIED: systemd-analyze is unavailable; no real systemd service was started")


if __name__ == "__main__":
    main()
