---
name: learn-cloud-platform-engineering
description: Build project-based learning paths for distributed systems, Kubernetes, cloud infrastructure, platform engineering, SRE, security, observability, and infrastructure as code. Use when assessing current skill gaps, sequencing labs, turning a repository into a curriculum, preparing for platform or cloud roles, or defining measurable learning milestones.
---

# Learn Cloud Platform Engineering

Build competence through observable system behavior, not topic checklists.

## Workflow

1. Inspect the learner's repository, tools, and stated target role. Distinguish implemented behavior from files that merely mention a technology.
2. Create a capability baseline across distributed correctness, Kubernetes, cloud primitives, IaC, delivery, observability, security, and operations.
3. Choose one thin end-to-end workload that remains consistent across the curriculum.
4. Use `cloud-docs` to retrieve current upstream and provider guidance for each milestone.
5. Define labs in increasing failure scope: process, pod, node, zone, and region.
6. Attach proof to every milestone: tests, metrics, traces, recovery results, architecture records, or deployable artifacts.
7. Reassess after each milestone and remove activities that only add tooling without new understanding.

Read [curriculum-rubric.md](references/curriculum-rubric.md) when producing a multi-week roadmap or evaluating whether a project demonstrates platform-engineering skill.
