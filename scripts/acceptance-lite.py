#!/usr/bin/env python3
"""Isolated Linux container acceptance; never run against a user's data directory.

Usage: python3 scripts/acceptance-lite.py --full /tmp/full --lite /tmp/lite
       --kernels /tmp/kernels --work /tmp/lite-acceptance
"""
import argparse
import hashlib
import http.cookiejar
import http.server
import json
import os
from pathlib import Path
import select
import signal
import socket
import sqlite3
import subprocess
import threading
import time
import urllib.error
import urllib.parse
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("full", "lite", "kernels", "work"):
        parser.add_argument("--" + name, required=True, type=Path)
    args = parser.parse_args()
    if not Path("/.dockerenv").exists():
        raise SystemExit("This fixture requires an isolated Docker container")
    args.work.mkdir(parents=True, exist_ok=False)
    # mihomo intentionally bypasses 127/8 before user routing. Give the fixture
    # an isolated address so a successful request really exercises PROXY rules.
    subprocess.run(["ip", "link", "add", "dr-lite-fixture", "type", "dummy"], check=True)
    subprocess.run(["ip", "link", "set", "dr-lite-fixture", "up"], check=True)
    for host in ("10.203.0.2", "10.203.0.3", "10.203.0.4"):
        subprocess.run(["ip", "addr", "add", host + "/32", "dev", "dr-lite-fixture"], check=True)
    counts = {"proxy": 0, "origin": 0}
    results = []

    def passed(name, detail=""):
        results.append({"test": name, "result": "PASS", "detail": detail})
        print("PASS", name, detail, flush=True)

    class Origin(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            counts["origin"] += 1
            body = b"duorelay-lite-origin-fixture"
            self.send_response(200)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *args):
            pass

    class Proxy(Origin):
        def do_CONNECT(self):
            counts["proxy"] += 1
            host, port = self.path.rsplit(":", 1)
            try:
                upstream = socket.create_connection((host, int(port)), 10)
            except OSError:
                self.send_error(502)
                return
            self.send_response(200)
            self.end_headers()
            try:
                while True:
                    ready, _, _ = select.select([self.connection, upstream], [], [], 15)
                    if not ready:
                        break
                    for src in ready:
                        data = src.recv(65536)
                        if not data:
                            return
                        (upstream if src is self.connection else self.connection).sendall(data)
            finally:
                upstream.close()

        def do_GET(self):
            counts["proxy"] += 1
            connection = urllib.request.build_opener(urllib.request.ProxyHandler({}))
            try:
                with connection.open(self.path, timeout=10) as response:
                    body = response.read()
                    self.send_response(response.status)
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
            except OSError:
                self.send_error(502)

    servers = []
    for port, handler in [(19091, Origin), (19192, Proxy), (19193, Proxy)]:
        server = http.server.ThreadingHTTPServer(("0.0.0.0", port), handler)
        server.daemon_threads = True
        threading.Thread(target=server.serve_forever, daemon=True).start()
        servers.append(server)
    cookies = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(cookies))
    base = "http://127.0.0.1:18080"
    process = None
    log = None
    db = args.work / "duorelay.db"
    configs = args.work / "configs"
    report = args.work / "results.json"

    def api(method, path, body=None, status=200):
        request = urllib.request.Request(base + path, method=method, data=None if body is None else json.dumps(body).encode(), headers={"Content-Type": "application/json"})
        try:
            response = client.open(request, timeout=90)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            data = response.read()
            assert response.code == status, (method, path, response.code, data)
            value = json.loads(data)
            return value.get("data") if value.get("success") else value

    def stop():
        nonlocal process, log
        if process is not None:
            process.send_signal(signal.SIGTERM)
            process.wait(timeout=20)
            assert process.returncode == 0, process.returncode
            process = None
            log.close()

    def start(binary, name):
        nonlocal process, log
        log = (args.work / (name + ".log")).open("wb")
        process = subprocess.Popen([str(binary), "-listen", "127.0.0.1:18080", "-db", str(db), "-config-dir", str(configs), "-kernels", str(args.kernels)], stdout=log, stderr=subprocess.STDOUT)
        deadline = time.monotonic() + 240
        while time.monotonic() < deadline:
            if process.poll() is not None:
                raise AssertionError((name, process.returncode, (args.work / (name + ".log")).read_text()))
            try:
                api("GET", "/api/auth/status")
                return
            except OSError:
                time.sleep(0.2)
        raise AssertionError("panel startup timed out: " + name)

    def login():
        api("POST", "/api/auth/login", {"username": "lite-fixture", "password": "temporary-lite-fixture-only-2026"})

    def snapshot():
        nodes = api("GET", "/api/nodes")
        for node in nodes:
            node.pop("status", None)
        return {"nodes": sorted(nodes, key=lambda n: n["id"]), "subscriptions": api("GET", "/api/subscriptions"), "rules": api("GET", "/api/config")["routing_rules"]}

    def database_payloads():
        with sqlite3.connect(db) as connection:
            return {name: connection.execute("SELECT * FROM " + name + " ORDER BY 1").fetchall() for name in ("nodes", "subscriptions", "subscription_nodes")}

    def request_through_kernel(host):
        return subprocess.run(["curl", "--silent", "--show-error", "--fail", "--max-time", "15", "--noproxy", "", "--proxy", "http://127.0.0.1:17890", f"http://{host}:19091/check"], capture_output=True, text=True)

    try:
        start(args.full, "full-before")
        assert api("GET", "/api/auth/status")["configured"] is False
        api("POST", "/api/auth/register", {"username": "lite-fixture", "password": "temporary-lite-fixture-only-2026"}, status=201)
        login()
        passed("full first setup with temporary administrator")
        ids = []
        for name, port in [("entry", 19192), ("exit", 19193)]:
            node = api("POST", "/api/nodes", {"name": name, "protocol": "http", "server": {"host": "127.0.0.1", "port": port}}, status=201)
            ids.append(node["id"])
        exclusive = []
        for protocol in ("shadowtls", "naive", "tor", "openconnect"):
            node = api("POST", "/api/nodes", {"name": protocol, "protocol": protocol, "server": {"host": "example.invalid", "port": 443}}, status=201)
            exclusive.append(node["id"])
        api("POST", "/api/subscriptions", {"name": "temporary-disabled", "url": "https://example.invalid/temporary-token", "disabled": True}, status=201)
        settings = api("GET", "/api/config")
        settings.update({"kernel": "sing-box", "mixed_port": 17890, "inbound_port": 17891, "routing_mode": "global", "default_outbound": "PROXY", "transparent_mode": "off", "system_proxy": False,
                         "routing_rules": [
                             {"id": "proxy", "source": "user", "enabled": True, "match": "ip_cidr", "values": ["10.203.0.2/32"], "outbound": "PROXY"},
                             {"id": "direct", "source": "user", "enabled": True, "match": "ip_cidr", "values": ["10.203.0.3/32"], "outbound": "DIRECT"},
                             {"id": "reject", "source": "user", "enabled": True, "match": "ip_cidr", "values": ["10.203.0.4/32"], "outbound": "REJECT"}]})
        api("PUT", "/api/config", settings)
        baseline = snapshot()
        stop()
        durable = database_payloads()
        key_hash = hashlib.sha256(Path(str(db) + ".key").read_bytes()).hexdigest()

        start(args.lite, "lite")
        login()
        assert api("GET", "/api/auth/variant") == {"variant": "lite", "kernels": ["mihomo"]}
        assert api("GET", "/api/build")["variant"] == "lite"
        with client.open(base + "/") as response:
            assert response.status == 200 and b"<html" in response.read()
        assert snapshot() == baseline
        passed("full -> lite: six nodes, encrypted subscription and three rules identical")
        geo = {p.name: {"bytes": p.stat().st_size, "sha256": hashlib.sha256(p.read_bytes()).hexdigest()} for p in configs.iterdir() if p.name in ("geoip.metadb", "GeoSite.dat")}
        assert len(geo) == 2 and all(v["bytes"] > 0 for v in geo.values())
        passed("first lite startup downloads official Geo files", json.dumps(geo))
        for node_id in exclusive:
            api("POST", f"/api/nodes/{node_id}/activate", {}, status=400)
        api("POST", "/api/config/apply", {"kernel": "mihomo", "config": "proxies: []"}, status=400)
        passed("lite rejects exclusive protocols and raw YAML before apply")
        api("POST", f"/api/nodes/{ids[0]}/activate", {})
        status = api("GET", "/api/kernel/status")
        assert status["running"] and status["active_kernel"] == "mihomo", status
        # The shared generator's manual PROXY selector initially selects DIRECT.
        # Exercise its actual proxy branch explicitly, using this temporary
        # database's Clash controller credentials (no raw config edits).
        with sqlite3.connect(db) as connection:
            current = json.loads(connection.execute("SELECT value FROM backend_state WHERE key='current_config'").fetchone()[0])
        selection = urllib.request.Request(current["controller"] + "/proxies/PROXY", method="PUT", data=b'{"name":"entry"}', headers={"Content-Type": "application/json", "Authorization": "Bearer " + current["secret"]})
        with client.open(selection) as response:
            assert response.status == 204
        count = counts["proxy"]
        proxied = request_through_kernel("10.203.0.2")
        assert proxied.returncode == 0 and "duorelay-lite-origin-fixture" in proxied.stdout and counts["proxy"] > count, (proxied, counts)
        passed("real HTTP request through panel-managed mihomo and temporary upstream", str(counts))
        count = counts["proxy"]
        direct = request_through_kernel("10.203.0.3")
        assert direct.returncode == 0 and "duorelay-lite-origin-fixture" in direct.stdout and counts["proxy"] == count
        rejected = request_through_kernel("10.203.0.4")
        assert rejected.returncode != 0 and counts["proxy"] == count
        passed("routing: PROXY / DIRECT / REJECT observed independently")
        old = api("GET", "/api/config/current")
        api("POST", "/api/config/apply", {"kernel": "sing-box", "node_ids": ids[:1]}, status=400)
        assert api("GET", "/api/config/current")["id"] == old["id"]
        assert request_through_kernel("10.203.0.2").returncode == 0
        passed("invalid replacement preserves running instance")
        bad = api("POST", "/api/nodes", {"name": "temporary-invalid-native-cipher", "protocol": "shadowsocks", "server": {"host": "127.0.0.1", "port": 19192}, "proto": {"method": "duorelay-invalid-cipher", "password": "temporary"}}, status=201)
        failure = api("POST", f"/api/nodes/{bad['id']}/activate", {}, status=422)
        assert failure["error"]["code"] == "KERNEL_CHECK_FAILED", failure
        assert api("GET", "/api/config/current")["id"] == old["id"]
        assert request_through_kernel("10.203.0.2").returncode == 0
        api("DELETE", f"/api/nodes/{bad['id']}")
        passed("real mihomo validator rejects invalid cipher without disrupting prior instance")
        for mode in ("load_balance", "chain"):
            state = api("POST", "/api/config/apply", {"mode": mode, "node_ids": ids})
            assert state["kernel"] == "mihomo" and state["mode"] == mode
            count = counts["proxy"]
            request = request_through_kernel("10.203.0.2")
            assert request.returncode == 0 and counts["proxy"] > count, (mode, request, counts)
            if mode == "chain":
                assert counts["proxy"] - count >= 2, ("chain did not traverse both temporary proxies", counts)
            passed("real " + mode + " request through native mihomo")
        api("POST", "/api/kernel/stop", {})
        assert snapshot() == baseline
        stop()
        assert database_payloads() == durable
        assert hashlib.sha256(Path(str(db) + ".key").read_bytes()).hexdigest() == key_hash
        passed("lite does not rewrite node/subscription payloads or database key")

        start(args.full, "full-after")
        login()
        assert snapshot() == baseline
        assert api("GET", "/api/config")["kernel"] == "sing-box"
        passed("lite -> full: original preference, nodes, subscriptions, rules preserved")
        stop()

        # Fresh lite database also completes initial setup; an existing cache
        # avoids network dependence on every later start.
        db = args.work / "fresh-lite.db"
        before_geo = {p: p.stat().st_mtime_ns for p in configs.iterdir() if p.name in geo}
        start(args.lite, "lite-fresh")
        assert not api("GET", "/api/auth/status")["configured"]
        api("POST", "/api/auth/register", {"username": "lite-fixture", "password": "temporary-lite-fixture-only-2026"}, status=201)
        login()
        assert api("GET", "/api/config")["kernel"] == "mihomo"
        assert all(p.stat().st_mtime_ns == timestamp for p, timestamp in before_geo.items())
        passed("fresh lite setup and cached Geo reuse")
        fresh_ids = []
        for name, port in [("fresh-entry", 19192), ("fresh-exit", 19193)]:
            node = api("POST", "/api/nodes", {"name": name, "protocol": "http", "server": {"host": "127.0.0.1", "port": port}}, status=201)
            fresh_ids.append(node["id"])
        api("POST", "/api/subscriptions", {"name": "lite-created", "url": "https://example.invalid/lite-created-token", "disabled": True}, status=201)
        fresh_settings = api("GET", "/api/config")
        fresh_settings.update({"mixed_port": 17890, "inbound_port": 17891, "system_proxy": False, "routing_rules": baseline["rules"]})

        def network_state():
            return [subprocess.check_output(command, text=True) for command in (
                ["ip", "-details", "rule", "show"], ["ip", "-details", "route", "show", "table", "all"], ["nft", "list", "ruleset"]) ]

        for transparent_mode in ("redirect", "tproxy", "tun"):
            for composition in ("single", "chain"):
                before = network_state()
                fresh_settings.update({"transparent_mode": transparent_mode, "enable_tun": transparent_mode == "tun"})
                api("PUT", "/api/config", fresh_settings)
                if composition == "single":
                    api("POST", f"/api/nodes/{fresh_ids[0]}/activate", {})
                else:
                    api("POST", "/api/config/apply", {"mode": "chain", "node_ids": fresh_ids})
                assert api("GET", "/api/kernel/status")["running"]
                assert api("GET", "/api/config/current")["transparent_mode"] == transparent_mode
                if transparent_mode in ("redirect", "tproxy"):
                    table = "ip" if transparent_mode == "redirect" else "inet"
                    rules = subprocess.check_output(["nft", "list", "table", table, "duorelay"], text=True)
                    assert ("redirect" if transparent_mode == "redirect" else "tproxy") in rules
                api("POST", "/api/kernel/stop", {})
                assert network_state() == before, (transparent_mode, composition, "network state not restored")
                passed("panel " + transparent_mode + " / " + composition + " startup and network cleanup (not transparent traffic)")
        fresh_baseline = snapshot()
        stop()
        start(args.full, "full-from-lite-created")
        login()
        assert snapshot() == fresh_baseline
        assert api("GET", "/api/config")["kernel"] == "mihomo"
        passed("lite-created nodes, subscription and rules reopen in full unchanged")
        stop()
    finally:
        if process is not None:
            stop()
        for server in servers:
            server.shutdown()
        subprocess.run(["ip", "link", "delete", "dr-lite-fixture"], check=True)
        report.write_text(json.dumps(results, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
