"""Regression: a failed partial goose migration must never restore/restart old code."""
import pathlib
import subprocess
import unittest

SCRIPT = pathlib.Path(__file__).resolve().parents[1] / "scripts" / "update-test-runtime.sh"


class PartialMigrationRollback(unittest.TestCase):
    def test_unchanged_goose_version_still_blocks_rollback(self):
        text = SCRIPT.read_text()
        remote = text.split("<<'REMOTE_DEPLOY'\n", 1)[1].split("\nREMOTE_DEPLOY", 1)[0]
        start = remote.index("rollback() {")
        end = remote.index("\nfailure() {", start)
        rollback_fn = remote[start:end]
        # This simulates a migration that created a table but failed before
        # recording a goose version: schema_before and schema_after both 12.
        shell = (
            "set -euo pipefail\n"
            "migration_attempted=1\nschema_before=12\n"
            "systemctl() { echo \"systemctl:$*\" >> \"$TRACE\"; }\n"
            "install() { echo install >> \"$TRACE\"; }\n"
            "runuser() { echo 12; }\n"
            + rollback_fn + "\n"
            + 'if rollback; then exit 42; fi\n'
            + 'test "$(cat "$TRACE")" = "systemctl:stop bfe-chat-service.service"\n'
        )
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            trace = pathlib.Path(tmp) / "calls"
            result = subprocess.run(
                ["bash", "-c", shell],
                env={"TRACE": str(trace), "PATH": "/usr/bin:/bin"},
                text=True, capture_output=True, check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("automatic rollback forbidden", result.stderr)
            self.assertNotIn("install", trace.read_text())
            self.assertNotIn("restart", trace.read_text())

    def test_migration_marker_precedes_service_restart(self):
        remote = SCRIPT.read_text().split("<<'REMOTE_DEPLOY'\n", 1)[1].split("\nREMOTE_DEPLOY", 1)[0]
        marker = remote.index("migration_attempted=1", remote.index("trap failure ERR"))
        restart = remote.index("systemctl restart bfe-chat-service.service", marker)
        self.assertLess(marker, restart)
        self.assertIn('if [[ "$migration_attempted" -eq 1 ]]', remote)


if __name__ == "__main__":
    unittest.main()
