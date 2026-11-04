#!/usr/bin/env python3
"""Run full/lite API and real SOCKS traffic acceptance in an isolated Linux directory.

python3 scripts/acceptance-api.py --full /tmp/full --lite /tmp/lite --kernels ./kernels
This fixture needs no host networking privileges. It never uses an existing database.
"""
import argparse
import hashlib
import http.cookiejar
import http.server
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import socketserver
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.request


def exact(connection, count):
    data = b""
    while len(data) < count:
        chunk = connection.recv(count - len(data))
        if not chunk:
            raise ConnectionError("unexpected SOCKS EOF")
        data += chunk
    return data


class Upstream(socketserver.BaseRequestHandler):
    def handle(self):
        connection = self.request
        connection.settimeout(10)
        try:
            version, methods = exact(connection, 2)
            assert version == 5 and 0 in exact(connection, methods)
            connection.sendall(b"\x05\x00")
            version, command, _, address_type = exact(connection, 4)
            assert version == 5 and command == 1
            count = {1: 4, 4: 16}.get(address_type)
            if address_type == 3:
                count = exact(connection, 1)[0]
            assert count
            exact(connection, count + 2)
            connection.sendall(b"\x05\x00\x00\x01\x7f\x00\x00\x01\x00\x00")
            headers = b""
            while b"\r\n\r\n" not in headers and len(headers) < 32768:
                headers += exact(connection, 1)
            body = self.server.marker.encode()
            connection.sendall(b"HTTP/1.1 200 OK\r\nConnection: close\r\nContent-Length: " + str(len(body)).encode() + b"\r\n\r\n" + body)
        except (ConnectionError, OSError):
            pass


class SocksServer(socketserver.ThreadingTCPServer):
    daemon_threads = True
    allow_reuse_address = True


class Origin(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = b"resolved-by-fixture-dns"
        self.send_response(200)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


class DNS(socketserver.BaseRequestHandler):
    def handle(self):
        data, connection = self.request
        if len(data) < 17:
            return
        self.server.queries += 1
        # Copy the question and return a single loopback A record. AAAA queries
        # receive an empty successful answer, with no external DNS involved.
        answer = b""
        if data[-4:-2] == b"\x00\x01":
            answer = b"\xc0\x0c\x00\x01\x00\x01\x00\x00\x00\x3c\x00\x04\x7f\x00\x00\x01"
        reply = data[:2] + b"\x81\x80\x00\x01" + (b"\x00\x01" if answer else b"\x00\x00") + b"\x00\x00\x00\x00" + data[12:] + answer
        connection.sendto(reply, self.client_address)


def free_port():
    with socket.socket() as connection:
        connection.bind(("127.0.0.1", 0))
        return connection.getsockname()[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--full", type=Path, required=True)
    parser.add_argument("--lite", type=Path, required=True)
    parser.add_argument("--kernels", type=Path, required=True)
    parser.add_argument("--geodata", type=Path, help="optional directory of real mihomo Geo files")
    parser.add_argument("--work", type=Path, help="new directory for logs/results; otherwise use /tmp")
    args = parser.parse_args()
    if os.name != "posix" or not Path("/proc/sys/kernel/ostype").read_text().startswith("Linux"):
        raise SystemExit("Run this fixture on Linux")
    for name in ("full", "lite", "kernels"):
        setattr(args, name, getattr(args, name).resolve())
    if args.work:
        work = args.work.resolve()
        work.mkdir(parents=True, exist_ok=False)
    else:
        work = Path(tempfile.mkdtemp(prefix="duorelay-acceptance-"))
    print("Acceptance artifacts:", work, flush=True)
    results = []
    servers = []
    process = log = None
    base = ""
    cookies = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(cookies))

    def passed(name):
        print("PASS", name, flush=True)
        results.append({"test": name, "result": "PASS"})

    def api(method, path, body=None, status=200):
        request = urllib.request.Request(base + path, method=method, data=None if body is None else json.dumps(body).encode(), headers={"Content-Type": "application/json"})
        try:
            response = client.open(request, timeout=120)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read()
            assert response.code == status, (method, path, response.code, raw.decode())
            value = json.loads(raw)
            return value.get("data") if value.get("success") else value

    def stop():
        nonlocal process, log
        if process is not None:
            process.send_signal(signal.SIGTERM)
            try:
                code = process.wait(timeout=30)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
                raise AssertionError("panel did not shut down")
            finally:
                process = None
                log.close()
            assert code == 0, (code, log.name)

    def start(binary, database, configs, name):
        nonlocal process, log, base
        port = free_port()
        base = f"http://127.0.0.1:{port}"
        configs.mkdir(parents=True, exist_ok=True)
        if args.geodata:
            for filename in ("GeoSite.dat", "geoip.metadb"):
                shutil.copy2(args.geodata / filename, configs / filename)
        log = (work / (name + ".log")).open("wb")
        process = subprocess.Popen([str(binary), "--listen", f"127.0.0.1:{port}", "--db", str(database), "--config-dir", str(configs), "--kernels", str(args.kernels)], stdout=log, stderr=subprocess.STDOUT)
        deadline = time.monotonic() + 240
        while time.monotonic() < deadline:
            if process.poll() is not None:
                raise AssertionError(Path(log.name).read_text())
            try:
                api("GET", "/api/auth/status")
                return
            except OSError:
                time.sleep(0.2)
        raise AssertionError("panel startup timed out: " + str(log.name))

    def traffic(port, marker):
        deadline = time.monotonic() + 3
        while True:
            result = subprocess.run(["curl", "--noproxy", "", "--silent", "--show-error", "--fail", "--max-time", "12", "--socks5-hostname", f"127.0.0.1:{port}", "http://203.0.113.10/fixture"], capture_output=True, text=True)
            if result.returncode == 0 or time.monotonic() >= deadline:
                break
            time.sleep(0.1)  # Wait only for the newly started kernel listener.
        assert result.returncode == 0 and result.stdout == marker, (port, result.returncode, result.stdout, result.stderr)

    try:
        links = []
        for marker in ("node-A", "node-B"):
            server = SocksServer(("127.0.0.1", 0), Upstream)
            server.marker = marker
            threading.Thread(target=server.serve_forever, daemon=True).start()
            servers.append(server)
            links.append(f"socks5://127.0.0.1:{server.server_address[1]}#same-name")
        socks_servers = servers[:]
        origin = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Origin)
        dns = socketserver.ThreadingUDPServer(("127.0.0.1", 0), DNS)
        dns.daemon_threads = True
        dns.queries = 0
        for server in (origin, dns):
            threading.Thread(target=server.serve_forever, daemon=True).start()
            servers.append(server)
        for variant, binary, kernels in (("full", args.full, ("sing-box", "mihomo")), ("lite", args.lite, ("mihomo",))):
            directory = work / variant
            directory.mkdir()
            database, configs = directory / "duorelay.db", directory / "configs"
            start(binary, database, configs, variant)
            api("POST", "/api/auth/register", {"username": "admin", "password": "fixture-password-2026"}, 201)
            api("POST", "/api/auth/login", {"username": "admin", "password": "fixture-password-2026"})
            report = api("POST", "/api/nodes/import", {"content": "\n".join(links + [links[0], "unknown://hidden@host:443"])}, 201)
            assert report["added"] == 2 and report["duplicates"] == 1 and len(report["skipped"]) == 1, report
            nodes = api("GET", "/api/nodes")
            nodes.sort(key=lambda node: node["server"]["port"])
            # Associate by endpoint; display names deliberately remain identical.
            nodes = [next(node for node in nodes if node["server"]["port"] == server.server_address[1]) for server in socks_servers]
            assert all(node["name"] == "same-name" for node in nodes)
            api("PUT", "/api/nodes/" + nodes[0]["id"], {"tags": ["edited"], "favorite": True})
            report = api("POST", "/api/nodes/import", {"content": "\n".join(links)}, 201)
            assert report["added"] == 0 and report["duplicates"] == 2
            passed(variant + ": partial import, deduplication, duplicate display names, partial edit")
            ports = [free_port(), free_port()]
            while ports[0] == ports[1]:
                ports[1] = free_port()
            settings = api("GET", "/api/config")
            settings.update({"inbound_port": free_port(), "mixed_port": free_port(), "socks_port": free_port(), "dns_servers": [f"udp://127.0.0.1:{dns.server_address[1]}"], "dns_detour": "DIRECT", "routing_mode": "blacklist", "default_outbound": "DIRECT", "socks_listeners": [{"port": port, "node_id": node["id"]} for port, node in zip(ports, nodes)]})
            for kernel in kernels:
                settings["kernel"] = kernel
                api("PUT", "/api/config", settings)
                api("POST", "/api/config/apply", {"kernel": kernel, "node_ids": [node["id"] for node in nodes]})
                for port, marker in zip(ports, ("node-A", "node-B")):
                    traffic(port, marker)
                # Repeat after readiness: mappings must remain independent of
                # the main DIRECT route and the primary selector.
                for port, marker in zip(ports, ("node-A", "node-B")):
                    traffic(port, marker)
                api("POST", "/api/nodes/batch-delete", {"node_ids": [node["id"] for node in nodes]}, 409)
                passed(variant + ": " + kernel + " real traffic through two SOCKS ports and deletion protection")
                previous_queries = dns.queries
                proxy_port = settings["inbound_port"] if kernel == "sing-box" else settings["mixed_port"]
                result = subprocess.run(["curl", "--noproxy", "", "--silent", "--show-error", "--fail", "--max-time", "12", "--proxy", f"http://127.0.0.1:{proxy_port}", f"http://{variant}-{kernel}.duorelay.invalid:{origin.server_address[1]}/dns"], capture_output=True, text=True)
                assert result.returncode == 0 and result.stdout == "resolved-by-fixture-dns" and dns.queries > previous_queries, (result.stdout, result.stderr, dns.queries)
                passed(variant + ": " + kernel + " resolves a fresh domain through configured DNS")
            # Rejected permission preflight must leave the running kernel intact.
            invalid = dict(settings, transparent_mode="invalid")
            api("PUT", "/api/config", invalid, 400)
            traffic(ports[1], "node-B")
            api("PUT", "/api/kernel/logs/policy", {"retention_days": 3, "cleanup_hours": 2})
            logs = api("GET", "/api/kernel/logs?kernel=" + kernels[-1])
            assert len(logs) <= 100
            api("POST", "/api/kernel/stop")
            settings["socks_listeners"] = []
            api("PUT", "/api/config", settings)
            before = api("GET", "/api/nodes")
            api("POST", "/api/nodes/batch-delete", {"node_ids": [nodes[0]["id"], "missing-node"]}, 404)
            assert len(api("GET", "/api/nodes")) == len(before)
            backup = directory / "snapshot"
            result = subprocess.run([str(binary), "--db", str(database), "--backup", str(backup)], capture_output=True, text=True)
            assert result.returncode == 0, result.stderr
            manifest = json.loads((backup / "manifest.json").read_text())
            for filename in ("duorelay.db", "duorelay.db.key"):
                assert hashlib.sha256((backup / filename).read_bytes()).hexdigest() == manifest[filename + ".sha256"]
            result = subprocess.run([str(binary), "--db", str(database), "--reset-password"], input="reset-fixture-2026\n", capture_output=True, text=True)
            assert result.returncode == 0, result.stderr
            api("GET", "/api/nodes", status=401)
            api("POST", "/api/auth/login", {"username": "admin", "password": "reset-fixture-2026"})
            api("POST", "/api/nodes/batch-delete", {"node_ids": [node["id"] for node in nodes]})
            assert api("GET", "/api/nodes") == []
            api("DELETE", "/api/kernel/logs?kernel=" + kernels[-1])
            stop()
            cookies.clear()
            start(binary, backup / "duorelay.db", directory / "restored-configs", variant + "-restored")
            api("POST", "/api/auth/login", {"username": "admin", "password": "fixture-password-2026"})
            restored = api("GET", "/api/nodes")
            assert len(restored) == 2 and any(node.get("favorite") for node in restored)
            assert api("GET", "/api/kernel/logs/policy")["cleanup_hours"] == 2
            stop()
            # Maintenance must not silently create a replacement encryption key.
            (backup / "duorelay.db.key").unlink()
            result = subprocess.run([str(binary), "--db", str(backup / "duorelay.db"), "--reset-password"], input="reset-fixture-2026\n", capture_output=True, text=True)
            assert result.returncode != 0 and not (backup / "duorelay.db.key").exists()
            passed(variant + ": atomic deletion, log policy persistence, live backup/restore, password reset and missing-key protection")
            # Bind failure is detected before startup or initialization mutates data.
            with socket.socket() as occupied:
                occupied.bind(("127.0.0.1", 0))
                occupied.listen()
                result = subprocess.run([str(binary), "--listen", "127.0.0.1:" + str(occupied.getsockname()[1]), "--db", str(directory / "unused.db")], capture_output=True, text=True, timeout=10)
                assert result.returncode != 0 and "unavailable" in result.stderr and not (directory / "unused.db").exists()
            passed(variant + ": occupied HTTP port fails before database creation")
    finally:
        stop()
        for server in servers:
            server.shutdown()
            server.server_close()
        (work / "results.json").write_text(json.dumps(results, indent=2) + "\n")


if __name__ == "__main__":
    main()
