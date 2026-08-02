#!/usr/bin/env python3

import re
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[3]
JENKINS_DIR = ROOT / "platform" / "jenkins"


class JenkinsContractTests(unittest.TestCase):
    def test_versions_are_pinned(self) -> None:
        values = {}
        for line in (JENKINS_DIR / "versions.lock").read_text(encoding="utf-8").splitlines():
            if line and not line.startswith("#"):
                key, value = line.split("=", 1)
                values[key] = value
        self.assertRegex(values["JENKINS_CHART_VERSION"], r"^\d+\.\d+\.\d+$")
        self.assertNotIn("latest", "\n".join(values.values()))
        self.assertRegex(values["PYTHON_IMAGE"], r"@sha256:[0-9a-f]{64}$")
        self.assertRegex(values["DOCKER_CLI_AMD64_IMAGE"], r"@sha256:[0-9a-f]{64}$")
        self.assertRegex(values["DOCKER_CLI_ARM64_IMAGE"], r"@sha256:[0-9a-f]{64}$")
        self.assertRegex(values["DOCKER_DIND_AMD64_SOURCE"], r"@sha256:[0-9a-f]{64}$")
        self.assertRegex(values["DOCKER_DIND_ARM64_SOURCE"], r"@sha256:[0-9a-f]{64}$")
        self.assertRegex(values["JENKINS_KIND_NODE_IMAGE"], r"@sha256:[0-9a-f]{64}$")

    def test_host_preflight_enforces_pinned_tool_versions(self) -> None:
        text = (JENKINS_DIR / "scripts" / "preflight.sh").read_text(
            encoding="utf-8"
        )
        self.assertIn('source "${script_dir}/../versions.lock"', text)
        self.assertIn('"${installed_kind_version}" != "${KIND_VERSION}"', text)
        self.assertIn('"${installed_helm_version}" =~ ^v3\\.', text)

    def test_controller_and_agent_security(self) -> None:
        values = yaml.safe_load(
            (JENKINS_DIR / "helm" / "values.yaml").read_text(encoding="utf-8")
        )
        controller = values["controller"]
        self.assertEqual(controller["numExecutors"], 0)
        self.assertTrue(controller["admin"]["createSecret"])
        self.assertEqual(controller["admin"]["existingSecret"], "jenkins-admin")
        self.assertTrue(controller["disableRememberMe"])
        self.assertFalse(controller["installLatestPlugins"])
        self.assertFalse(controller["legacyRemotingSecurityEnabled"])
        self.assertIn("timestamper:1.30", controller["installPlugins"])
        self.assertFalse(values["rbac"]["readSecrets"])
        self.assertFalse(values["serviceAccountAgent"]["automountServiceAccountToken"])
        self.assertTrue(values["networkPolicy"]["enabled"])
        self.assertTrue(values["persistence"]["enabled"])

    def test_privilege_is_limited_to_integration_agent(self) -> None:
        values = yaml.safe_load(
            (JENKINS_DIR / "helm" / "values.yaml").read_text(encoding="utf-8")
        )
        templates = values["agent"]["podTemplates"]
        self.assertNotIn("privileged: true", templates["verification"])
        self.assertIn("privileged: true", templates["integration"])
        self.assertEqual(templates["verification"].count("agentInjection: true"), 1)
        self.assertEqual(templates["integration"].count("agentInjection: true"), 1)
        self.assertIn("agentContainer: jnlp", templates["verification"])
        self.assertIn("agentContainer: jnlp", templates["integration"])
        self.assertIn("automountServiceAccountToken: false", templates["verification"])
        self.assertIn("automountServiceAccountToken: false", templates["integration"])
        self.assertIn('resourceRequestMemory: "8Gi"', templates["integration"])
        self.assertIn('resourceLimitMemory: "10Gi"', templates["integration"])
        self.assertIn("activeDeadlineSeconds: 7200", templates["integration"])

    def test_jenkinsfile_uses_only_static_agent_labels(self) -> None:
        text = (ROOT / "Jenkinsfile").read_text(encoding="utf-8")
        self.assertIn("jenkins-verify", text)
        self.assertIn("jenkins-integration", text)
        for forbidden in ("agent any", "podTemplate(", "withCredentials(", "credentials("):
            self.assertNotIn(forbidden, text)
        self.assertEqual(
            set(re.findall(r"stage\('(PS[0-2])'\)", text)),
            {"PS0", "PS1", "PS2"},
        )
        self.assertEqual(text.count("retry(3)"), 4)
        self.assertIn("timeout(time: 120, unit: 'MINUTES')", text)
        self.assertEqual(
            text.count('test "$(git rev-parse HEAD)" = "$EXPECTED_COMMIT"'),
            3,
        )
        self.assertIn(
            "git fetch --no-tags --unshallow origin +refs/heads/main:", text
        )

    def test_job_scm_is_shallow_and_source_branch_bounded(self) -> None:
        text = (JENKINS_DIR / "helm" / "values.yaml").read_text(encoding="utf-8")
        self.assertIn(
            "refspec('+refs/heads/' + '$' + '{SOURCE_BRANCH}:"
            "refs/remotes/origin/' + '$' + '{SOURCE_BRANCH}')",
            text,
        )
        self.assertIn("branch('*/' + '$' + '{SOURCE_BRANCH}')", text)
        self.assertIn("shallow(true)", text)
        self.assertIn("noTags(true)", text)
        self.assertIn("depth(1)", text)
        self.assertIn("timeout(5)", text)
        self.assertIn("honorRefspec(true)", text)
        self.assertIn("stringParam('EXPECTED_COMMIT'", text)
        self.assertIn("stringParam('SOURCE_BRANCH'", text)

    def test_agent_build_uses_a_narrow_temporary_context(self) -> None:
        text = (JENKINS_DIR / "scripts" / "build-agent.sh").read_text(encoding="utf-8")
        self.assertIn("jenkins-agent-context.", text)
        self.assertIn('"${build_context}"', text)
        self.assertIn("for attempt in 1 2 3", text)
        self.assertNotIn('  "${repo_root}"\n', text)

    def test_bootstrap_preloads_required_agent_images(self) -> None:
        text = (JENKINS_DIR / "scripts" / "bootstrap.sh").read_text(encoding="utf-8")
        self.assertIn('kind load docker-image "${JENKINS_AGENT_IMAGE}"', text)
        self.assertIn('dind_source="${DOCKER_DIND_ARM64_SOURCE}"', text)
        self.assertIn('dind_source="${DOCKER_DIND_AMD64_SOURCE}"', text)
        self.assertIn('docker pull "${dind_source}"', text)
        self.assertIn('docker tag "${dind_source}" "${DOCKER_DIND_IMAGE}"', text)
        self.assertIn('kind load docker-image "${DOCKER_DIND_IMAGE}"', text)
        self.assertIn("for attempt in 1 2", text)
        self.assertIn("if (( jenkins_ready == 0 ))", text)
        self.assertNotIn('kind load docker-image "${JENKINS_CONTROLLER_IMAGE}"', text)

    def test_trigger_keeps_credentials_out_of_process_arguments(self) -> None:
        text = (JENKINS_DIR / "scripts" / "trigger-and-wait.sh").read_text(
            encoding="utf-8"
        )
        self.assertIn("--netrc-file", text)
        self.assertIn("--cookie-jar", text)
        self.assertIn("--dump-header", text)
        self.assertIn("/queue/item/", text)
        self.assertNotIn("/lastBuild/", text)
        self.assertNotIn('--user "admin:${admin_password}"', text)
        self.assertIn("buildWithParameters", text)
        self.assertIn('--data-urlencode "EXPECTED_COMMIT=${expected_commit}"', text)
        self.assertIn('--data-urlencode "SOURCE_BRANCH=${source_branch}"', text)
        self.assertIn("check-ref-format --branch", text)
        self.assertIn("pipeline passed for %s", text)
        self.assertIn('JENKINS_BUILD_TIMEOUT_SECONDS:-7800', text)
        self.assertIn("while (( SECONDS < build_deadline ))", text)

    def test_agent_base_image_arguments_are_global(self) -> None:
        text = (JENKINS_DIR / "agent" / "Dockerfile").read_text(encoding="utf-8")
        lines = text.splitlines()
        first_from = next(index for index, line in enumerate(lines) if line.startswith("FROM "))
        self.assertIn("ARG DOCKER_CLI_IMAGE", lines[:first_from])
        self.assertIn("ARG PYTHON_IMAGE", lines[:first_from])
        self.assertIn("gcc git jq libc6-dev", text)
        self.assertIn('test "$(go env CGO_ENABLED)" = 1', text)
        self.assertIn("procps ripgrep tar", text)
        self.assertIn("openjdk-21-jre-headless", text)

    def test_ephemeral_vm_capacity_and_cleanup_are_explicit(self) -> None:
        text = (JENKINS_DIR / "scripts" / "run-ephemeral.sh").read_text(
            encoding="utf-8"
        )
        self.assertIn('JENKINS_COLIMA_CPUS:-8', text)
        self.assertIn('JENKINS_COLIMA_MEMORY_GIB:-16', text)
        self.assertIn('colima delete "${profile}" --force --data', text)
        self.assertIn("trap 'cleanup 129' HUP", text)
        self.assertIn("trap 'cleanup 130' INT", text)
        self.assertIn("trap 'cleanup 143' TERM", text)

    def test_ps2_uses_a_bounded_single_node_ci_cluster(self) -> None:
        config = yaml.safe_load(
            (ROOT / "platform" / "local" / "kind" / "ci-cluster.yaml").read_text(
                encoding="utf-8"
            )
        )
        self.assertEqual(
            [node["role"] for node in config["nodes"]],
            ["control-plane"],
        )
        local_config = yaml.safe_load(
            (ROOT / "platform" / "local" / "kind" / "cluster.yaml").read_text(
                encoding="utf-8"
            )
        )
        self.assertEqual(
            [node["role"] for node in local_config["nodes"]],
            ["control-plane", "worker", "worker"],
        )
        text = (ROOT / "scripts" / "ci" / "presubmit-ps2.sh").read_text(
            encoding="utf-8"
        )
        self.assertIn("kind/ci-cluster.yaml", text)


if __name__ == "__main__":
    unittest.main()
