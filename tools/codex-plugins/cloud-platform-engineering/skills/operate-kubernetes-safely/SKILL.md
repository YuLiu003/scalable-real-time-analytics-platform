---
name: operate-kubernetes-safely
description: Diagnose and operate Kubernetes clusters and workloads with evidence-first, least-destructive procedures. Use for CrashLoopBackOff, Pending pods, failed rollouts, service or DNS failures, resource pressure, scheduling, storage, networking, RBAC, certificates, autoscaling, Kafka-on-Kubernetes incidents, or requests involving kubectl changes to a live cluster.
---

# Operate Kubernetes Safely

Preserve evidence before changing state. A successful restart is not a root-cause diagnosis.

## Incident workflow

1. Confirm cluster, context, namespace, time window, impact, and recent changes.
2. Gather read-only evidence across desired state, observed state, events, logs, metrics, endpoints, and dependency health.
3. Build and rank falsifiable hypotheses. Identify the observation that would confirm or reject each one.
4. Use `cloud-docs` for exact API behavior, managed-service constraints, and version-specific remediation.
5. Choose the smallest reversible mitigation that reduces impact while preserving diagnostic evidence.
6. Obtain authority before destructive, security-sensitive, or production-mutating actions.
7. Verify user-visible recovery, data behavior, rollout health, and recurrence signals.
8. Record root cause, contributing conditions, detection gap, and durable corrective action.

Never expose Secret contents, tokens, kubeconfigs, or cloud credentials. Avoid deleting pods, scaling stateful services, force-deleting resources, editing finalizers, or restarting broad scopes until evidence and impact justify it.

Read [incident-playbook.md](references/incident-playbook.md) before proposing live mutations or writing a runbook.
