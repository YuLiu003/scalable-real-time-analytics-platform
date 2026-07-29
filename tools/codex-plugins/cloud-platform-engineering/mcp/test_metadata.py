#!/usr/bin/env python3

import json
import unittest
from pathlib import Path

import yaml


PLUGIN_ROOT = Path(__file__).resolve().parent.parent
SKILLS_ROOT = PLUGIN_ROOT / "skills"


def read_yaml(path: Path) -> dict:
    value = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise AssertionError(f"{path} must contain a YAML object")
    return value


def read_skill(path: Path) -> tuple[dict, str]:
    text = path.read_text(encoding="utf-8")
    parts = text.split("---", 2)
    if len(parts) != 3 or parts[0].strip():
        raise AssertionError(f"{path} must start with YAML frontmatter")
    frontmatter = yaml.safe_load(parts[1])
    if not isinstance(frontmatter, dict):
        raise AssertionError(f"{path} frontmatter must be a YAML object")
    return frontmatter, parts[2]


class PluginMetadataTests(unittest.TestCase):
    def test_manifest_paths_exist(self) -> None:
        manifest = json.loads(
            (PLUGIN_ROOT / ".codex-plugin" / "plugin.json").read_text(encoding="utf-8")
        )
        self.assertEqual(manifest["name"], PLUGIN_ROOT.name)
        self.assertTrue((PLUGIN_ROOT / manifest["skills"]).is_dir())
        self.assertTrue((PLUGIN_ROOT / manifest["mcpServers"]).is_file())

        mcp = json.loads((PLUGIN_ROOT / manifest["mcpServers"]).read_text(encoding="utf-8"))
        for definition in mcp["mcpServers"].values():
            command_path = PLUGIN_ROOT / definition["cwd"] / definition["args"][0]
            self.assertTrue(command_path.is_file())

    def test_skill_metadata_is_complete_and_consistent(self) -> None:
        skill_directories = sorted(path for path in SKILLS_ROOT.iterdir() if path.is_dir())
        self.assertTrue(skill_directories)

        for skill_directory in skill_directories:
            with self.subTest(skill=skill_directory.name):
                skill_path = skill_directory / "SKILL.md"
                frontmatter, body = read_skill(skill_path)
                self.assertEqual(set(frontmatter), {"name", "description"})
                self.assertEqual(frontmatter["name"], skill_directory.name)
                self.assertTrue(frontmatter["description"].strip())
                self.assertNotIn("[TODO", body)

                agent = read_yaml(skill_directory / "agents" / "openai.yaml")["interface"]
                self.assertTrue(agent["display_name"].strip())
                self.assertGreaterEqual(len(agent["short_description"]), 25)
                self.assertLessEqual(len(agent["short_description"]), 64)
                self.assertIn(f"${skill_directory.name}", agent["default_prompt"])


if __name__ == "__main__":
    unittest.main()
