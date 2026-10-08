#!/usr/bin/env python3
import copy
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("boundary", Path(__file__).with_name("check-provisioning-boundary.py"))
boundary = importlib.util.module_from_spec(spec)
spec.loader.exec_module(boundary)


class BoundaryTests(unittest.TestCase):
    def setUp(self):
        self.config = {"services": {
            "api": {"environment": {"WPMGR_PROVISIONING_ENABLED": "true"}},
            "provisioning-worker": {"volumes": [{"source": "/var/run/docker.sock", "target": "/var/run/docker.sock"},
                {"type": "bind", "source": "/var/lib/wpmgr/provisioning", "target": "/var/lib/wpmgr/provisioning"}],
                "cap_drop": ["ALL"], "cap_add": ["CHOWN"],
                "environment": {"WPMGR_PROVISIONING_MARIADB_PASSWORD": "test-secret",
                    "WPMGR_PROVISIONING_STATE_DIR": "/var/lib/wpmgr/provisioning"}},
        }}

    def test_valid(self):
        boundary.check(self.config)

    def test_api_nonsecret_snapshot(self):
        self.config["services"]["api"]["environment"].update({
            "WPMGR_PROVISIONING_MARIADB_ADDRESS": "host.docker.internal:3306",
            "WPMGR_PROVISIONING_WORDPRESS_IMAGE": "wordpress:6.8.2-php8.3-apache",
            "WPMGR_PROVISIONING_PROXY_NETWORK": "proxy",
        })
        boundary.check(self.config)

    def test_username_leak(self):
        self.config["services"]["api"]["environment"]["WPMGR_PROVISIONING_MARIADB_USERNAME"] = "provisioner"
        with self.assertRaises(ValueError):
            boundary.check(self.config)

    def test_socket_leak(self):
        self.config["services"]["api"]["volumes"] = copy.deepcopy(self.config["services"]["provisioning-worker"]["volumes"])
        with self.assertRaises(ValueError):
            boundary.check(self.config)

    def test_secret_leak(self):
        self.config["services"]["api"]["environment"]["WPMGR_PROVISIONING_MARIADB_PASSWORD"] = "test-secret"
        with self.assertRaises(ValueError):
            boundary.check(self.config)

    def test_missing_services(self):
        with self.assertRaises(ValueError):
            boundary.check({})

    def test_published_ports(self):
        self.config["services"]["provisioning-worker"]["ports"] = [{"target": 80}]
        with self.assertRaises(ValueError):
            boundary.check(self.config)

    def test_privileged(self):
        self.config["services"]["provisioning-worker"]["privileged"] = True
        with self.assertRaises(ValueError):
            boundary.check(self.config)

    def test_missing_chown(self):
        self.config["services"]["provisioning-worker"]["cap_add"] = []
        with self.assertRaisesRegex(ValueError, "CHOWN"):
            boundary.check(self.config)

    def test_broad_capability(self):
        self.config["services"]["provisioning-worker"]["cap_add"].append("SYS_ADMIN")
        with self.assertRaisesRegex(ValueError, "CHOWN"):
            boundary.check(self.config)

    def test_missing_capability_drop(self):
        self.config["services"]["provisioning-worker"]["cap_drop"] = []
        with self.assertRaisesRegex(ValueError, "CHOWN"):
            boundary.check(self.config)

    def test_relative_state_dir(self):
        self.config["services"]["provisioning-worker"]["environment"]["WPMGR_PROVISIONING_STATE_DIR"] = "./state"
        with self.assertRaisesRegex(ValueError, "absolute"):
            boundary.check(self.config)

    def test_mismatched_state_mount(self):
        self.config["services"]["provisioning-worker"]["volumes"][1]["source"] = "/different/path"
        with self.assertRaisesRegex(ValueError, "identical"):
            boundary.check(self.config)

    def test_missing_state_mount(self):
        self.config["services"]["provisioning-worker"]["volumes"].pop()
        with self.assertRaisesRegex(ValueError, "identical"):
            boundary.check(self.config)


if __name__ == "__main__":
    unittest.main()
