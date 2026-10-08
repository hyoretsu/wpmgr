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
            "provisioning-worker": {"volumes": [{"source": "/var/run/docker.sock", "target": "/var/run/docker.sock"}],
                "environment": {"WPMGR_PROVISIONING_MARIADB_PASSWORD": "test-secret"}},
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


if __name__ == "__main__":
    unittest.main()
