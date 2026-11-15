import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

CHECK = Path(__file__).with_name("check-verification-refs.py").resolve()


class VerificationRefTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.git("init", "-q")
        self.git("config", "user.email", "test@example.invalid")
        self.git("config", "user.name", "Test")
        self.git("commit", "--allow-empty", "-qm", "base")
        self.base = self.git("rev-parse", "HEAD").strip()
        self.git("commit", "--allow-empty", "-qm", "feature")
        self.feature = self.git("rev-parse", "HEAD").strip()
        self.guide = self.root / ".agents/skills/verify-example/SKILL.md"
        self.guide.parent.mkdir(parents=True)

    def git(self, *args):
        return subprocess.check_output(["git", *args], cwd=self.root, text=True)

    def check(self, text):
        self.guide.write_text(text)
        return subprocess.run([sys.executable, str(CHECK), self.base], cwd=self.root, text=True, capture_output=True)

    def test_base_commit_is_allowed_in_prose_and_command(self):
        result = self.check(f"Matched to `{self.base}`.\ngit diff {self.base} -- source\n")
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_existing_feature_commit_is_rejected(self):
        result = self.check(f"Matched to `{self.feature}`.\n")
        self.assertEqual(result.returncode, 1)
        self.assertIn("use a verified commit reachable from the target branch", result.stdout)

    def test_missing_commit_in_copyable_command_is_rejected(self):
        result = self.check(f"Matched to `{self.base}`.\ngit diff 888a135 -- source\n")
        self.assertEqual(result.returncode, 1)
        self.assertIn("SKILL.md:2: 888a135", result.stdout)


if __name__ == "__main__":
    unittest.main()
