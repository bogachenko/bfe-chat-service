"""Independent regression tests for staged QR-08 coverage reporting."""
import tempfile
import unittest
from pathlib import Path

from coverage_report import percent, read


class CoverageProfileTests(unittest.TestCase):
    def parse(self, profile):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "coverage.out"
            path.write_text(profile)
            return read(path)

    def test_deduplicates_across_test_binaries_in_either_order(self):
        block = "github.com/bogachenko/bfe-chat-service/internal/storage/a.go:1.1,2.1"
        other = "github.com/bogachenko/bfe-chat-service/internal/storage/b.go:1.1,2.1"
        for hit_sequence in ((1, 0), (0, 1)):
            with self.subTest(hit_sequence=hit_sequence):
                profile = ("mode: atomic\n"
                           + "".join(f"{block} 5 {hit}\n" for hit in hit_sequence)
                           + f"{other} 5 0\n")
                rows = self.parse(profile)
                self.assertEqual(len(rows), 2)
                self.assertEqual(percent(rows), 50.0)

    def test_duplicate_cannot_inflate_total_or_hit(self):
        block = "github.com/bogachenko/bfe-chat-service/internal/authorization/a.go:1.1,2.1"
        rows = self.parse("mode: atomic\n" + f"{block} 7 1\n" * 5)
        self.assertEqual(len(rows), 1)
        self.assertEqual(percent(rows), 100.0)

    def test_rejects_inconsistent_duplicate(self):
        block = "github.com/bogachenko/bfe-chat-service/internal/storage/a.go:1.1,2.1"
        with self.assertRaisesRegex(ValueError, "inconsistent"):
            self.parse(f"mode: atomic\n{block} 3 1\n{block} 4 0\n")

    def test_rejects_bad_profile_and_empty(self):
        for profile in ("", "mode: invalid\n", "mode: atomic\n", "mode: atomic\ninvalid 1 0\n"):
            with self.subTest(profile=profile), self.assertRaises(ValueError):
                self.parse(profile)


if __name__ == "__main__":
    unittest.main()
