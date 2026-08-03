from __future__ import annotations

import unittest

from agent_review_optimizer.routing import route_changed_files


class RoutingTests(unittest.TestCase):
    def test_routes_every_supported_specialty_without_disclosing_paths(self) -> None:
        plan = route_changed_files(
            [
                "services/market-pipeline/main.go",
                "tools/analyze.py",
                "contracts/events/new.schema.json",
                "platform/gitops/apps/kustomization.yaml",
                "infra/opentofu/aws/main.tf",
                ".github/workflows/presubmit.yml",
                "Jenkinsfile",
                "contracts/fixtures/demo.json",
                "infra/example.tofu",
                "services/market-pipeline/main.go",
                "",
            ]
        )
        self.assertEqual(plan.changed_file_count, 9)
        self.assertEqual(
            plan.reviewers,
            (
                "general",
                "go",
                "python",
                "event-driven",
                "kubernetes",
                "cloud-iac",
                "ci-security",
                "privacy-finance",
            ),
        )
        rendered = plan.to_mapping()
        self.assertEqual(rendered["schema_version"], 1)
        self.assertNotIn("paths", rendered)

    def test_routes_documentation_to_general_reviewer(self) -> None:
        self.assertEqual(route_changed_files(["README.md"]).reviewers, ("general",))

    def test_routes_jenkins_platform_to_ci_security(self) -> None:
        self.assertEqual(
            route_changed_files(["platform/jenkins/agent/Dockerfile"]).reviewers,
            ("general", "ci-security"),
        )

    def test_routes_both_sides_of_a_rename(self) -> None:
        plan = route_changed_files(
            ["infra/opentofu/aws/main.tf", "docs/retired-infrastructure.txt"]
        )
        self.assertEqual(plan.reviewers, ("general", "cloud-iac"))

    def test_rejects_missing_or_unsafe_paths_without_echoing_them(self) -> None:
        for paths in ([], [""], ["/private/file"], ["docs/../secret"], ["bad\0path"]):
            with self.subTest(paths=paths), self.assertRaisesRegex(
                ValueError, "relative repository paths|at least one"
            ) as raised:
                route_changed_files(paths)
            for path in paths:
                if path:
                    self.assertNotIn(path, str(raised.exception))


if __name__ == "__main__":
    unittest.main()
