"""Static safety checks for isolated Chat test VPS provisioning."""
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]
PROVISION = ROOT / "deploy/scripts/provision-test-runtime.sh"
WRAPPER = ROOT / "deploy/scripts/run-provision-test-runtime.sh"
NGINX = ROOT / "deploy/nginx/bfe-chat.conf"


class ProvisionContract(unittest.TestCase):
    def test_approval_guards(self):
        self.assertIn("BFE_CHAT_PROVISION_APPROVED", PROVISION.read_text())
        self.assertIn("BFE_CHAT_TEST_PROVISION_APPROVED", WRAPPER.read_text())

    def test_vps_identity_pinned(self):
        code = WRAPPER.read_text()
        self.assertIn("machine_id_sha256", code)
        self.assertIn("StrictHostKeyChecking=yes", code)

    def test_database_isolation(self):
        script = PROVISION.read_text()
        self.assertIn('CREATE DATABASE bfe_chat', script)
        self.assertIn('CREATE ROLE "bfe-chat"', script)
        self.assertNotIn("REVOKE CONNECT ON DATABASE bfe_drive", script)
        self.assertNotIn("ALTER DATABASE bfe_drive", script)

    def test_dedicated_vhost(self):
        config = NGINX.read_text()
        self.assertIn("server_name chat.webshopstudio.ru;", config)
        self.assertIn("proxy_pass http://127.0.0.1:8081;", config)
        self.assertIn("location /ws {", config)
        self.assertIn("location /v1/ {", config)
        self.assertNotIn("drive.webshopstudio.ru", config)

    def test_interrupted_acme_vhost_is_exactly_checked(self):
        script = PROVISION.read_text()
        self.assertIn("is_chat_acme_vhost()", script)
        self.assertIn('cmp -s "$nginx_target" <(', script)
        self.assertIn("is_chat_acme_vhost || {", script)

    def test_first_deployment_without_goose_table(self):
        script = (ROOT / "deploy/scripts/update-test-runtime.sh").read_text()
        self.assertIn("to_regclass('public.goose_db_version')", script)
        self.assertIn('schema_before="$(chat_schema_version)"', script)
        self.assertIn('schema_after="$(chat_schema_version 2>/dev/null)"', script)
        self.assertNotIn("NOT has_database_privilege('bfe-chat','bfe_drive','CONNECT')", script)

    def test_deploy_waits_for_listener_readiness(self):
        script = (ROOT / "deploy/scripts/update-test-runtime.sh").read_text()
        self.assertIn("for attempt in {1..60}; do", script)
        self.assertIn("systemctl is-active --quiet bfe-chat-service.service || break", script)
        self.assertIn('grep -Fq "pid=$main_pid,"', script)
        self.assertIn('[[ "$ready" -eq 1 ]]', script)
        self.assertIn("migration_attempted=1", script)

    def test_no_chat_start_or_migrations(self):
        script = PROVISION.read_text()
        self.assertNotIn("systemctl start bfe-chat", script)
        self.assertNotIn("systemctl restart bfe-chat", script)
        self.assertNotIn("goose up", script)


if __name__ == "__main__":
    unittest.main()
