#!/usr/bin/env python3

import os
import re
import subprocess
import tempfile
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[3]
JENKINS_DIR = ROOT / "platform" / "jenkins"


class JenkinsContractTests(unittest.TestCase):
    def test_versions_are_pinned(self) -> None:
        values = {}
        for line in (
            (JENKINS_DIR / "versions.lock").read_text(encoding="utf-8").splitlines()
        ):
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
        for key in ("GARAGE_AMD64_SOURCE", "GARAGE_ARM64_SOURCE"):
            self.assertRegex(
                values[key], r"^dxflrs/garage:v2\.3\.0@sha256:[0-9a-f]{64}$"
            )
        for key in ("SOCAT_AMD64_SOURCE", "SOCAT_ARM64_SOURCE"):
            self.assertRegex(
                values[key], r"^alpine/socat:[^@]+@sha256:[0-9a-f]{64}$"
            )
        self.assertNotEqual(values["GARAGE_AMD64_SOURCE"], values["GARAGE_ARM64_SOURCE"])
        self.assertNotEqual(values["SOCAT_AMD64_SOURCE"], values["SOCAT_ARM64_SOURCE"])
        self.assertEqual(values["GARAGE_IMAGE"], "local/jenkins-garage:2.3.0")
        self.assertEqual(values["SOCAT_IMAGE"], "local/jenkins-socat:1.8.0.3")
        self.assertRegex(
            values["JENKINS_ARTIFACT_MANAGER_S3_VERSION"], r"^[0-9]+\.v"
        )

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
        self.assertIn(
            "artifact-manager-s3:973.v6a_51253e896b_",
            controller["installPlugins"],
        )
        self.assertEqual(
            controller["nodeSelector"]["platform.local/role"],
            "jenkins-control",
        )
        self.assertEqual(
            controller["tolerations"],
            [
                {
                    "key": "node-role.kubernetes.io/control-plane",
                    "operator": "Exists",
                    "effect": "NoSchedule",
                }
            ],
        )
        self.assertFalse(values["rbac"]["readSecrets"])
        self.assertFalse(values["serviceAccountAgent"]["automountServiceAccountToken"])
        self.assertTrue(values["networkPolicy"]["enabled"])
        self.assertTrue(values["persistence"]["enabled"])
        self.assertEqual(
            values["persistence"]["existingClaim"], "jenkins-retained-home"
        )

    def test_artifacts_use_dedicated_garage_with_bounded_retention(self) -> None:
        values_text = (JENKINS_DIR / "helm" / "values.yaml").read_text(
            encoding="utf-8"
        )
        pipeline = (ROOT / "Jenkinsfile").read_text(encoding="utf-8")
        garage = yaml.safe_load_all(
            (JENKINS_DIR / "storage" / "garage.yaml").read_text(encoding="utf-8")
        )
        garage_documents = list(garage)
        statefulset = next(
            item for item in garage_documents if item["kind"] == "StatefulSet"
        )
        pod_spec = statefulset["spec"]["template"]["spec"]
        image = pod_spec["containers"][0]["image"]
        artifact_bootstrap = (
            JENKINS_DIR / "scripts" / "bootstrap-artifact-store.sh"
        ).read_text(encoding="utf-8")
        lifecycle_job = yaml.safe_load(
            (JENKINS_DIR / "storage" / "lifecycle-job.yaml").read_text(
                encoding="utf-8"
            )
        )

        self.assertIn("daysToKeep(3)", values_text)
        self.assertIn("numToKeep(20)", values_text)
        self.assertIn("daysToKeepStr: '3'", pipeline)
        self.assertIn("numToKeepStr: '20'", pipeline)
        self.assertIn('container: "jenkins-artifacts"', values_text)
        self.assertIn('prefix: "jenkins/"', values_text)
        self.assertIn('credentialsId: "jenkins-artifacts"', values_text)
        self.assertIn("scope: SYSTEM", values_text)
        self.assertNotIn("scope: GLOBAL", values_text)
        self.assertIn('customEndpoint: "127.0.0.1:13900"', values_text)
        self.assertIn('region: "us-east-1"', values_text)
        self.assertIn('customSigningRegion: "us-east-1"', values_text)
        self.assertIn('s3_region = "us-east-1"', artifact_bootstrap)
        self.assertIn(
            "<customSigningRegion>us-east-1</customSigningRegion>",
            (JENKINS_DIR / "scripts" / "verify.sh").read_text(encoding="utf-8"),
        )
        self.assertIn("disableSessionToken: true", values_text)
        self.assertEqual(image, "local/jenkins-garage:2.3.0")
        self.assertEqual(pod_spec["containers"][0]["imagePullPolicy"], "Never")
        self.assertEqual(values_text.count("image: local/jenkins-socat:1.8.0.3"), 3)
        self.assertEqual(values_text.count("imagePullPolicy: Never"), 3)
        self.assertEqual(
            lifecycle_job["spec"]["template"]["spec"]["securityContext"],
            {"seccompProfile": {"type": "RuntimeDefault"}},
        )
        self.assertNotIn("market-raw", values_text)
        self.assertNotIn("minio", values_text.lower())
        self.assertNotIn("minio", image.lower())
        self.assertEqual(
            pod_spec["tolerations"],
            [
                {
                    "key": "node-role.kubernetes.io/control-plane",
                    "operator": "Exists",
                    "effect": "NoSchedule",
                }
            ],
        )

    def test_retained_volumes_are_host_bounded_and_not_agent_mounted(self) -> None:
        cluster = yaml.safe_load(
            (JENKINS_DIR / "kind" / "cluster.yaml").read_text(encoding="utf-8")
        )
        mounts = cluster["nodes"][0]["extraMounts"]
        self.assertEqual(len(mounts), 1)
        self.assertEqual(
            mounts[0]["containerPath"],
            "/var/local/investment-platform/jenkins-retained",
        )
        self.assertFalse(mounts[0]["readOnly"])

        volumes = list(
            yaml.safe_load_all(
                (JENKINS_DIR / "storage" / "volumes.yaml").read_text(
                    encoding="utf-8"
                )
            )
        )
        persistent_volumes = [
            item for item in volumes if item["kind"] == "PersistentVolume"
        ]
        self.assertEqual(len(persistent_volumes), 2)
        for volume in persistent_volumes:
            self.assertEqual(volume["spec"]["persistentVolumeReclaimPolicy"], "Retain")
            self.assertTrue(
                volume["spec"]["hostPath"]["path"].startswith(
                    "/var/local/investment-platform/jenkins-retained/"
                )
            )

        values_text = (JENKINS_DIR / "helm" / "values.yaml").read_text(
            encoding="utf-8"
        )
        self.assertNotIn(
            "/var/local/investment-platform/jenkins-retained",
            values_text,
        )

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
        for forbidden in (
            "agent any",
            "podTemplate(",
            "withCredentials(",
            "credentials(",
        ):
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
        self.assertEqual(text.count("checkoutExpectedRevision()"), 4)
        self.assertIn("branches: [[name: env.EXPECTED_COMMIT]]", text)
        self.assertIn("refs/heads/${env.SOURCE_BRANCH}", text)
        self.assertIn(
            "git fetch --no-tags --unshallow origin +refs/heads/main:", text
        )

    def test_job_uses_a_trusted_pipeline_and_bounded_source_checkout(self) -> None:
        text = (JENKINS_DIR / "helm" / "values.yaml").read_text(encoding="utf-8")
        self.assertIn(
            "refspec('+refs/heads/' + '$' + '{TRUSTED_PIPELINE_BRANCH}:"
            "refs/remotes/origin/' + '$' + '{TRUSTED_PIPELINE_BRANCH}')",
            text,
        )
        self.assertIn(
            "branch('*/' + '$' + '{TRUSTED_PIPELINE_BRANCH}')", text
        )
        self.assertIn("shallow(true)", text)
        self.assertIn("noTags(true)", text)
        self.assertIn("depth(1)", text)
        self.assertIn("timeout(5)", text)
        self.assertIn("honorRefspec(true)", text)
        self.assertIn("stringParam('EXPECTED_COMMIT'", text)
        self.assertIn("stringParam('SOURCE_BRANCH'", text)
        self.assertIn("stringParam('TRUSTED_PIPELINE_BRANCH', 'main'", text)
        self.assertIn("lightweight(false)", text)

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
        self.assertIn('garage_source="${GARAGE_ARM64_SOURCE}"', text)
        self.assertIn('garage_source="${GARAGE_AMD64_SOURCE}"', text)
        self.assertIn('socat_source="${SOCAT_ARM64_SOURCE}"', text)
        self.assertIn('socat_source="${SOCAT_AMD64_SOURCE}"', text)
        self.assertIn('docker pull "${garage_source}"', text)
        self.assertIn('docker tag "${garage_source}" "${GARAGE_IMAGE}"', text)
        self.assertIn('docker pull "${socat_source}"', text)
        self.assertIn('docker tag "${socat_source}" "${SOCAT_IMAGE}"', text)
        self.assertIn('kind load docker-image "${GARAGE_IMAGE}"', text)
        self.assertIn('kind load docker-image "${SOCAT_IMAGE}"', text)
        self.assertIn('bootstrap-artifact-store.sh', text)
        self.assertIn("jenkins-retained-lock-owner", text)
        self.assertIn("--from-file=owner-token=/dev/stdin", text)
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
        self.assertIn("port-forward service/jenkins-artifacts 13900:3900", text)
        self.assertIn(
            "successful Jenkins build retained no verification artifact", text
        )
        self.assertIn("outside dedicated Garage", text)
        self.assertIn("TRUSTED_PIPELINE_BRANCH=${trusted_pipeline_branch}", text)
        self.assertIn("--write-out '%{http_code}'", text)
        self.assertIn('"${artifact_http_code}" != "200"', text)

    def test_artifact_secrets_stay_out_of_process_arguments(self) -> None:
        text = (JENKINS_DIR / "scripts" / "bootstrap-artifact-store.sh").read_text(
            encoding="utf-8"
        )
        self.assertNotIn("--from-literal", text)
        self.assertIn('--from-file="garage.toml=${temporary_config_file}"', text)
        self.assertIn('--from-file="secret-key=${runtime_secret_file}"', text)

    def test_agent_base_image_arguments_are_global(self) -> None:
        text = (JENKINS_DIR / "agent" / "Dockerfile").read_text(encoding="utf-8")
        lines = text.splitlines()
        first_from = next(
            index for index, line in enumerate(lines) if line.startswith("FROM ")
        )
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
        self.assertIn("/var/local/investment-platform/jenkins-retained:w", text)
        self.assertIn("storage-metrics.py", text)
        self.assertIn("retained_state_dir", text)
        self.assertIn('caffeinate -dimsu -w "$$" &', text)
        self.assertIn('colima delete "${profile}" --force --data', text)
        self.assertIn('"${script_dir}/destroy.sh"', text)
        self.assertIn('"${script_dir}/retained-state.sh" acquire', text)
        self.assertIn("profile_lock_dir", text)
        self.assertIn("JENKINS_EXPECTED_COMMIT", text)
        self.assertIn('--commit "${expected_commit}"', text)
        self.assertIn("storage peak monitor exited unexpectedly", text)
        self.assertIn("trap 'cleanup 129' HUP", text)
        self.assertIn("trap 'cleanup 130' INT", text)
        self.assertIn("trap 'cleanup 143' TERM", text)

    def test_retained_state_purge_requires_marker_and_confirmation(self) -> None:
        text = (JENKINS_DIR / "scripts" / "retained-state.sh").read_text(
            encoding="utf-8"
        )
        self.assertIn("investment-platform-jenkins-retained-state-v1", text)
        self.assertIn("CONFIRM_JENKINS_RETAINED_PURGE", text)
        self.assertIn("refusing to purge an unmarked retained-state directory", text)
        self.assertIn('rm -rf -- "${state_dir}"', text)
        self.assertIn('"${state_dir%/}/"*', text)

    def test_retained_state_lock_and_canonical_path_behavior(self) -> None:
        script = JENKINS_DIR / "scripts" / "retained-state.sh"
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            fake_bin = root / "bin"
            fake_bin.mkdir()
            fake_openssl = fake_bin / "openssl"
            fake_openssl.write_text(
                "#!/bin/sh\n"
                "if [ \"$3\" = 16 ]; then\n"
                "  printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'\n"
                "else\n"
                "  printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
                "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'\n"
                "fi\n",
                encoding="utf-8",
            )
            fake_openssl.chmod(0o700)
            state = root / "state"
            environment = {
                **os.environ,
                "PATH": f"{fake_bin}:{os.environ['PATH']}",
                "JENKINS_RETAINED_STATE_DIR": str(state),
            }

            acquired = subprocess.run(
                (script, "acquire"),
                env=environment,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(acquired.returncode, 0, acquired.stderr)
            owner_environment = {
                **environment,
                "JENKINS_RETAINED_LOCK_TOKEN": acquired.stdout.strip(),
            }
            prepared = subprocess.run(
                (script, "prepare"),
                env=owner_environment,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(prepared.returncode, 0, prepared.stderr)
            self.assertFalse(
                (state / "secrets" / "rpc-secret").read_bytes().endswith(b"\n")
            )

            concurrent = subprocess.run(
                (script, "acquire"),
                env=environment,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertNotEqual(concurrent.returncode, 0)
            self.assertIn("already owned", concurrent.stderr)

            blocked_purge = subprocess.run(
                (script, "purge"),
                env={
                    **environment,
                    "CONFIRM_JENKINS_RETAINED_PURGE": "investment-platform-jenkins",
                },
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertNotEqual(blocked_purge.returncode, 0)
            self.assertTrue(state.is_dir())
            mismatched_release = subprocess.run(
                (script, "release"),
                env={
                    **environment,
                    "JENKINS_RETAINED_LOCK_TOKEN": "b" * 64,
                },
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertNotEqual(mismatched_release.returncode, 0)
            self.assertIn("owner token does not match", mismatched_release.stderr)
            self.assertTrue((state / ".active-lock").is_dir())
            subprocess.run((script, "release"), env=owner_environment, check=True)
            subprocess.run(
                (script, "purge"),
                env={
                    **environment,
                    "CONFIRM_JENKINS_RETAINED_PURGE": "investment-platform-jenkins",
                },
                check=True,
                stdout=subprocess.DEVNULL,
            )
            self.assertFalse(state.exists())

            repository_link = root / "repository-link"
            repository_link.symlink_to(ROOT, target_is_directory=True)
            linked_environment = {
                **environment,
                "JENKINS_RETAINED_STATE_DIR": str(repository_link / "retained"),
            }
            linked = subprocess.run(
                (script, "path"),
                env=linked_environment,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertNotEqual(linked.returncode, 0)
            self.assertIn("symbolic links", linked.stderr)

    def test_destroy_quiesces_retained_workloads_before_kind(self) -> None:
        destroy = (JENKINS_DIR / "scripts" / "destroy.sh").read_text(
            encoding="utf-8"
        )
        quiesce = (JENKINS_DIR / "scripts" / "quiesce.sh").read_text(
            encoding="utf-8"
        )
        self.assertLess(destroy.index("quiesce.sh"), destroy.index("kind delete"))
        self.assertLess(
            quiesce.index("scale_statefulset_if_present jenkins"),
            quiesce.index("jenkins-jenkins-agent=true"),
        )
        self.assertLess(
            quiesce.index("jenkins-jenkins-agent=true"),
            quiesce.index("scale_statefulset_if_present jenkins-artifacts"),
        )
        self.assertIn("--ignore-not-found --output=name", quiesce)
        self.assertIn("cannot list Jenkins pods during quiesce", quiesce)

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
