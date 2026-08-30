#!/usr/bin/env python3
import json
import os
import re
import subprocess
import sys
import urllib.parse

HERE = os.path.dirname(os.path.abspath(__file__))
DASHBOARD = os.path.join(HERE, "grafana", "dashboards", "vocalis.json")


def network():
    out = subprocess.run(
        ["docker", "network", "ls", "--format", "{{.Name}}"],
        capture_output=True, text=True,
    ).stdout.split()
    for n in out:
        if n.startswith("vocalis"):
            return n
    sys.exit("no vocalis docker network. Is the stack up? Run: make observe-up")


NET = network()


def fetch(host, path):
    r = subprocess.run(
        ["docker", "run", "--rm", "--network", NET, "curlimages/curl:latest",
         "-s", "--max-time", "20", host + path],
        capture_output=True, text=True,
    )
    try:
        return json.loads(r.stdout)
    except Exception:
        return {"status": "unreachable", "error": (r.stdout or r.stderr)[:80]}


def expand(expr):
    return (expr.replace("$container", "vocalis-.*")
                .replace("$__auto", "5m")
                .replace("$__interval", "5m"))


def count(panel, target):
    expr = expand(target["expr"])
    if panel["datasource"]["type"] == "prometheus":
        r = fetch("http://prometheus:9090", "/api/v1/query?query=" + urllib.parse.quote(expr))
    elif panel["type"] == "logs":
        r = fetch("http://loki:3100", "/loki/api/v1/query_range?limit=5&query=" + urllib.parse.quote(expr))
    else:
        r = fetch("http://loki:3100", "/loki/api/v1/query?query=" + urllib.parse.quote(expr))
    if r.get("status") != "success":
        return None, str(r.get("error"))[:70]
    return len(r.get("data", {}).get("result", [])), None


def blame(expr):
    if expr.startswith("node_") or "node_" in expr:
        return "node-exporter -- check `docker compose -f docker-compose.observe.yml logs node-exporter`"
    if "container_" in expr:
        return "cadvisor -- check that it is running and not --docker_only-filtered out"
    return "loki/promtail"


def main():
    dash = json.load(open(DASHBOARD))
    empty, broken = [], []

    print("Querying every panel on the Vocalis dashboard.\n")
    for panel in dash["panels"]:
        for target in panel.get("targets", []):
            n, err = count(panel, target)
            label = "%s [%s]" % (panel["title"], target["refId"])
            if err:
                print("  %-34s FAILED   %s" % (label, err))
                broken.append((label, expand(target["expr"])))
            elif n == 0:
                print("  %-34s empty" % label)
                empty.append((label, expand(target["expr"])))
            else:
                print("  %-34s ok       %d series" % (label, n))

    variable = dash["templating"]["list"][0]["query"]
    match = re.match(r"label_values\((.*),\s*(\w+)\)$", variable)
    r = fetch("http://prometheus:9090", "/api/v1/series?match[]=" + urllib.parse.quote(match.group(1)))
    names = sorted({s.get("name", "") for s in r.get("data", [])})
    print("\n  containers cadvisor can see: %s" % (", ".join(n for n in names if n) or "none"))

    if not empty and not broken:
        print("\nEvery panel has data.")
        return 0

    print("")
    for label, expr in broken + empty:
        print("  %s\n    %s\n    likely: %s\n" % (label, expr, blame(expr)))
    return 1


if __name__ == "__main__":
    sys.exit(main())
