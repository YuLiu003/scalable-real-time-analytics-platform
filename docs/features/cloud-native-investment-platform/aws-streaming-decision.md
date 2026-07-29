# AWS streaming and Kubernetes decision

| Field | Decision |
| --- | --- |
| Status | Accepted for the first billable AWS lab |
| Date | 2026-07-21 |
| Environment | Ephemeral `aws-lab`; no production claim |
| Kubernetes | Amazon EKS 1.35 with managed EC2 nodes |
| Kafka | Strimzi 1.1.0 / Kafka 4.2.1 on EKS |
| Cloud data services | S3, ECR, EBS CSI, KMS, CloudWatch, AWS Budgets |
| Workload identity | EKS Pod Identity |

## Context

The local kind environment already proves the producer, Kafka, archive,
analytics, API, replay, and crash boundaries. The next milestone must expose
provider-specific networking, identity, encryption, registry, storage, audit,
cost, and lifecycle behavior without changing the application contracts.

## Decision

Use a three-Availability-Zone EKS lab with one managed node group per zone and a
three-node Strimzi KRaft cluster.
Kafka topics use replication factor 3 and minimum in-sync replicas 2. Strimzi
uses zone rack awareness, required host anti-affinity, zone spread constraints,
and EBS CSI volumes with delayed binding. The lab runs three managed nodes
because useful Kafka quorum and zone exercises cannot be demonstrated on a
single node.

Use EKS Pod Identity associations for the archiver, analytics builder, and API.
Each service account receives a different S3/KMS policy, and each role's trust
policy restricts the namespace and service-account session tags. The
applications use the AWS SDK default credential chain in AWS while retaining
explicit static credentials only for the local S3-compatible Garage endpoint.
No AWS access key or secret is stored in Kubernetes desired state.

Use public worker subnets for this short-lived lab to avoid always-on NAT
gateways and multiple interface endpoints. Restrict the EKS public API to
operator CIDRs, require IMDSv2 with a one-hop limit, create no application
ingress, and record that this is not the production network pattern.

Separate the remote-state foundation from the lab lifecycle. The foundation
uses a KMS-encrypted, versioned S3 bucket and native S3 state locking; the lab
owns EKS, networking, data storage, registries, identities, logs, and budget.

## Rejected alternatives

### MSK first

MSK would reduce broker operations and is a valuable comparison, but it would
hide the Kubernetes scheduling, persistent-volume, disruption, certificate,
operator, and Kafka quorum work that this milestone is intended to teach. Add
an MSK root and measure responsibility, cost, authentication, networking, and
recovery after the Strimzi-on-EKS exercise has runtime evidence.

### Private nodes with NAT gateways or a complete endpoint set

This is the preferred production direction, but it adds recurring resources
whose cost can exceed the value of an occasional learning run. The lab chooses
explicitly constrained public nodes; a later production-like design must move
nodes private and validate ECR, S3, STS, EKS Auth, logs, and other endpoint or
egress requirements.

### Reusing static S3 keys

Rejected. Static keys would bypass the principal AWS identity lesson and create
secret distribution and rotation risks. AWS documents that Pod Identity maps an
IAM role to a Kubernetes service account and configures supported SDKs to obtain
temporary credentials through the Pod Identity agent.

## Failure and verification contract

- Losing one Kafka broker must retain quorum; this is not yet runtime-proven.
- Losing an entire zone is only a hypothesis until pod placement and EBS
  recovery are observed on AWS.
- The authorized archiver can write only bronze/quarantine prefixes.
- Analytics can read bronze and write silver/gold.
- The API can read only gold objects.
- A service account without an association must fail the S3 probe.
- Cluster creation and destruction require an expected-account check, a
  reviewed plan, and recorded orphan inspection.

Static validation and provider-schema checks are complete on this branch.
Runtime, failure, CloudTrail, and teardown evidence remain pending until an
explicitly authorized AWS apply.
