#!/usr/bin/env python3
"""Exposure scan (Story 1.7, AD-2/NFR-16, R-21): every internal port must be UNREACHABLE from the vantage point this runs from.
Run it from outside the host (public IP) and from a same-subnet non-allowlisted host (private IP) — both are mandatory gates
at pre-production; CI runs it against the runner's non-loopback address. Any reachable internal port => exit 1.
A refused/timed-out connect counts as unreachable; only a completed TCP handshake counts as exposed.
Usage: python3 -I tools/exposure_scan.py --target HOST [--target HOST2] [--ports 5001,5301,...] [--timeout 1.5]"""
import argparse, socket, sys
from concurrent.futures import ThreadPoolExecutor

# 5001 WK API, 5301 WK Manager, 7001 cluster RPC, 9099 WK Prometheus, 6060 pprof (conventional), 1883/1884 MQTT TCP/WS,
# 8080/9101 VeyroGram app + internal listener, 9102 IM webhook, 6379 Redis, 3306 MySQL.
DEFAULT_PORTS = [5001, 5301, 7001, 9099, 6060, 1883, 1884, 8080, 9101, 9102, 6379, 3306]

def probe(host, port, timeout):
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True
    except OSError:
        return False

def scan(targets, ports, timeout=1.5):
    jobs = [(t, p) for t in targets for p in ports]
    with ThreadPoolExecutor(max_workers=32) as ex:
        res = list(ex.map(lambda tp: probe(tp[0], tp[1], timeout), jobs))
    return [(t, p) for (t, p), open_ in zip(jobs, res) if open_]

def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--target', action='append', required=True)
    ap.add_argument('--ports', default=','.join(map(str, DEFAULT_PORTS)))
    ap.add_argument('--timeout', type=float, default=1.5)
    a = ap.parse_args(argv)
    ports = [int(p) for p in a.ports.split(',') if p]
    exposed = scan(a.target, ports, a.timeout)
    for t, p in exposed: print(f'EXPOSED: {t}:{p} accepted a TCP connection')
    print(f'exposure scan: {len(a.target)} target(s) x {len(ports)} port(s): ' + ('FAILED' if exposed else 'all unreachable'))
    return 1 if exposed else 0

if __name__ == '__main__':
    sys.exit(main())
