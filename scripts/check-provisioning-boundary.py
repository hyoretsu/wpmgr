#!/usr/bin/env python3
"""Validate rendered Compose configuration without printing environment secrets."""
import json
import sys


def check(config):
    services = config.get("services", {})
    worker = services.get("provisioning-worker")
    if not services or not worker or "api" not in services:
        raise ValueError("missing api or provisioning-worker service")
    for name, service in services.items():
        if name == "provisioning-worker":
            continue
        for mount in service.get("volumes", []):
            source = str(mount.get("source", ""))
            target = str(mount.get("target", ""))
            if source.endswith("docker.sock") or target.endswith("docker.sock"):
                raise ValueError("Docker socket exposed outside provisioning-worker")
        for key, value in service.get("environment", {}).items():
            if key in {
                "WPMGR_PROVISIONING_MARIADB_USERNAME",
                "WPMGR_PROVISIONING_MARIADB_PASSWORD",
            } and value:
                raise ValueError("MariaDB provisioning credential exposed outside worker")
    if worker.get("ports") or worker.get("privileged"):
        raise ValueError("worker must have no published ports or privileged flag")
    if not any(m.get("target") == "/var/run/docker.sock" for m in worker.get("volumes", [])):
        raise ValueError("worker Docker socket missing")


def main():
    try:
        check(json.load(sys.stdin))
    except (ValueError, TypeError, AttributeError) as error:
        print("provisioning boundary: FAIL: " + str(error), file=sys.stderr)
        return 1
    print("provisioning boundary: PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
