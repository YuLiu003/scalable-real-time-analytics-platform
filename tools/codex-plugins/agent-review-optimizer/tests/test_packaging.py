from __future__ import annotations

import json
import os
import unittest
from pathlib import Path


PLUGIN_ROOT = Path(__file__).resolve().parents[1]


class PackagingTests(unittest.TestCase):
    def test_manifest_skill_and_script_paths_resolve_inside_plugin(self) -> None:
        manifest = json.loads(
            (PLUGIN_ROOT / ".codex-plugin" / "plugin.json").read_text(
                encoding="utf-8"
            )
        )
        skills = (PLUGIN_ROOT / manifest["skills"]).resolve()
        script = (PLUGIN_ROOT / "scripts" / "review-optimizer").resolve()

        self.assertEqual(manifest["name"], PLUGIN_ROOT.name)
        self.assertEqual(skills.parent, PLUGIN_ROOT)
        self.assertEqual(
            list(skills.glob("*/SKILL.md")),
            [skills / "optimize-agent-reviews" / "SKILL.md"],
        )
        self.assertTrue(script.is_file())
        self.assertTrue(os.access(script, os.X_OK))


if __name__ == "__main__":
    unittest.main()
