"""Negative-scenario tests; never connects to a VPS."""
import importlib.util
import pathlib
import unittest

path = pathlib.Path(__file__).resolve().parents[1] / "scripts" / "check-test-runtime.py"
spec = importlib.util.spec_from_file_location("chat_preflight", path)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)

GOOD = """
http {
 server {
   listen 443 ssl;
   server_name chat.test.example;
   location /ws { proxy_pass http://127.0.0.1:8081; }
   location /v1/ { proxy_pass http://127.0.0.1:8081; }
   location /v2/ { proxy_pass http://127.0.0.1:8081; }
 }
}
"""
SS_GOOD = 'LISTEN 0 4096 127.0.0.1:8081 0.0.0.0:* users:(("bfe-chat-service",pid=137,fd=7))\n'


class NginxTests(unittest.TestCase):
    def test_valid_vhost(self):
        self.assertTrue(mod.nginx_ok(GOOD, "chat.test.example"))

    def test_split_vhost_rejected(self):
        bad = GOOD.replace("location /v1/ { proxy_pass http://127.0.0.1:8081; }", "")
        bad += "server { listen 443 ssl; server_name drive.test.example; location /v1/ { proxy_pass http://127.0.0.1:8081; } }"
        self.assertFalse(mod.nginx_ok(bad, "chat.test.example"))

    def test_missing_v2_rejected(self):
        bad = GOOD.replace("   location /v2/ { proxy_pass http://127.0.0.1:8081; }\\n", "")
        self.assertFalse(mod.nginx_ok(bad, "chat.test.example"))

    def test_plain_http_rejected(self):
        self.assertFalse(mod.nginx_ok(GOOD.replace("443 ssl", "80"), "chat.test.example"))

    def test_wrong_proxy_rejected(self):
        self.assertFalse(mod.nginx_ok(GOOD.replace("127.0.0.1:8081", "127.0.0.1:8080", 1), "chat.test.example"))

    def test_duplicate_hostname_rejected(self):
        self.assertFalse(mod.nginx_ok(GOOD + GOOD, "chat.test.example"))

    def test_unbalanced_config_rejected(self):
        with self.assertRaises(ValueError):
            mod.nginx_ok(GOOD + "{", "chat.test.example")


class ListenerTests(unittest.TestCase):
    def test_service_pid(self):
        self.assertTrue(mod.port_owned(SS_GOOD, "137"))

    def test_foreign_pid_rejected(self):
        self.assertFalse(mod.port_owned(SS_GOOD, "139"))

    def test_public_listener_rejected(self):
        self.assertFalse(mod.port_owned(SS_GOOD.replace("127.0.0.1", "0.0.0.0"), "137"))

    def test_multiple_listener_rejected(self):
        self.assertFalse(mod.port_owned(SS_GOOD * 2, "137"))

    def test_no_port_is_allowed_before_first_deploy(self):
        self.assertTrue(mod.port_owned("", "137"))


if __name__ == "__main__":
    unittest.main()
