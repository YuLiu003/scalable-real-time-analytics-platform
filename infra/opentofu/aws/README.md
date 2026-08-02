# AWS EKS platform lab

This directory is the first provider-specific cloud implementation of the
investment analytics platform. It is an ephemeral, billable learning lab—not a
production environment.

## What it demonstrates

```text
federated operator
       |
  OpenTofu state ---- encrypted/versioned S3 + native lock file
       |
  three-AZ VPC ---- restricted EKS public API
       |
  three zonal EKS managed node groups (1 node/AZ, Spot by default)
       |
  Strimzi Kafka 4.2.1 (3 KRaft nodes, RF=3, min ISR=2)
       |                         |
  Pod Identity                  EBS CSI / encrypted gp3
       |
  archiver -> encrypted S3 -> analytics -> Go API
       |
  immutable ECR images, control-plane logs, VPC flow logs, budget
```

AWS replaces only provider responsibilities: EKS replaces kind, EBS replaces
local persistent volumes, S3 replaces Garage, ECR replaces local image loading,
and EKS Pod Identity replaces static S3 credentials. Kafka remains under
Strimzi so this milestone demonstrates Kafka and Kubernetes operations. An MSK
comparison is intentionally deferred.

The lab uses one node group per Availability Zone because EBS volumes are
zonal and Kafka placement/recovery must preserve that boundary. It uses public
worker subnets to avoid NAT gateway and interface-endpoint
costs. Nodes receive public addresses but no application ingress is created,
the EC2 metadata service requires IMDSv2 with a hop limit of one, and the EKS
public endpoint requires explicit operator CIDRs. A production design would use
private node subnets, controlled egress or VPC endpoints, separate zonal node
groups for EBS-backed state, stronger network policy, backup, and tested DR.
Rack awareness, required host anti-affinity, and a `DoNotSchedule` zone-spread
constraint keep the three Kafka nodes on separate hosts and Availability Zones.

## Cost and authorization boundary

Amazon EKS, EC2, EBS, public IPv4 addresses, CloudWatch, KMS, S3, and ECR are
billable. As of 2026-07-21, AWS lists a standard-support EKS cluster at $0.10
per cluster-hour before worker and supporting-resource costs. Verify current
[EKS pricing](https://aws.amazon.com/eks/pricing/) before every apply.

The monthly budget is an alerting guardrail, not a hard real-time cap. AWS says
billing data and budget notifications can be delayed, so teardown remains an
operator responsibility. Configure `budget_alert_email` and accept the email
subscription before treating alerts as active. The `CostScope` cost-allocation
tag must also be activated in AWS Billing before the tag-filtered budget can
match spend, and activation is not retroactive. The launch template propagates
that tag to managed-node instances and root volumes, while the Kafka
StorageClass propagates it to dynamically provisioned EBS volumes. Inspect for
untagged provider-created resources during the runtime cost exercise.

Repository automation validates infrastructure but never applies it. The
manual `AWS Lab Plan` workflow obtains short-lived credentials through GitHub
OIDC and produces only a reviewable plan. `deploy-workloads.sh` also requires
the expected AWS account and an exact cluster-name confirmation.

## Prerequisites

- AWS account with billing access and a federated IAM role; do not create
  long-lived access keys for this lab.
- AWS CLI v2, OpenTofu 1.12.3, `kubectl`, Helm 3, Docker, and `jq`.
- A reviewed AWS Pricing Calculator estimate.
- An operator `/32` CIDR and the IAM role ARN that should receive the EKS access
  entry.

### GitHub OIDC plan boundary

Create a protected GitHub environment named `aws-lab-plan` and configure these
environment variables:

| Variable | Purpose |
| --- | --- |
| `AWS_ACCOUNT_ID` | Twelve-digit sandbox account guardrail |
| `AWS_REGION` | Lab and state region, for example `us-west-2` |
| `AWS_PLAN_ROLE_ARN` | Federated role used only by the plan workflow |
| `TOFU_STATE_BUCKET` | Bootstrap-created remote-state bucket |
| `TOFU_STATE_KMS_KEY_ARN` | Bootstrap-created state KMS key |
| `EKS_API_ALLOWED_CIDRS_JSON` | JSON list such as `["203.0.113.10/32"]` |
| `EKS_ADMIN_ROLE_ARN` | Federated role to receive the EKS access entry |
| `BUDGET_ALERT_EMAIL` | Optional budget subscriber address |

The AWS OIDC role trust must restrict both the audience and subject. Because
the workflow uses a GitHub environment, its expected subject is:

```text
repo:YuLiu003/scalable-real-time-analytics-platform:environment:aws-lab-plan
```

Restrict the trust policy to that exact value and `sts.amazonaws.com`. Give the
role only the read/list/describe permissions needed to refresh this stack plus
state-object read and KMS decrypt. Native state locking additionally needs
write/delete access to this stack's `.tflock` object and KMS encryption/data-key
use; it does not need infrastructure create, update, or delete permissions.
Protect the GitHub environment with required reviewers if the repository plan
supports them.

Run `AWS Lab Plan` manually after the state foundation exists. It verifies the
expected account, initializes encrypted remote state, creates a plan, and keeps
only the human-readable plan for seven days. It never runs `tofu apply` and
does not retain the binary plan artifact.

## 1. Create the remote-state foundation

The bootstrap state is independently owned and intentionally starts locally:

```bash
cd infra/opentofu/aws/bootstrap
cp terraform.tfvars.example terraform.tfvars
tofu init
tofu plan -out bootstrap.tfplan
# Review every create and the target account before explicitly applying.
tofu apply bootstrap.tfplan
```

Retain the bootstrap state securely. The state bucket uses KMS encryption,
versioning, native S3 lock files in downstream stacks, public-access blocking,
TLS enforcement, and `prevent_destroy`.

## 2. Plan the lab

```bash
cd ../lab
cp terraform.tfvars.example terraform.tfvars
tofu init \
  -backend-config="bucket=REPLACE_STATE_BUCKET" \
  -backend-config="key=aws-lab/platform.tfstate" \
  -backend-config="region=us-west-2" \
  -backend-config="kms_key_id=REPLACE_STATE_KMS_ARN"
tofu plan -out aws-lab.tfplan
tofu show aws-lab.tfplan
```

Review replacements, public routes and addresses, IAM policies, Kubernetes
version support tier, quotas, and forecast cost. Applying is a separate explicit
decision:

```bash
tofu apply aws-lab.tfplan
```

## 3. Publish and deploy immutable workloads

From the `lab` directory after a successful apply:

```bash
export AWS_REGION="$(tofu output -raw region)"
export S3_BUCKET="$(tofu output -raw analytics_bucket)"
export EKS_CLUSTER_NAME="$(tofu output -raw cluster_name)"
export EXPECTED_AWS_ACCOUNT_ID="$(tofu output -raw account_id)"
export ECR_REGISTRY="${EXPECTED_AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
export IMAGE_TAG="sha-$(git rev-parse --short=12 HEAD)"

../../../scripts/cloud/aws/publish-images.sh
export CONFIRM_AWS_LAB_DEPLOY="${EKS_CLUSTER_NAME}"
../../../scripts/cloud/aws/deploy-workloads.sh
```

The deployment installs Strimzi, reconciles a three-node Kafka cluster, copies
only the public broker CA into the application namespace, runs DEMO-ASSET-A, DEMO-ASSET-B,
DEMO-ASSET-C, and DEMO-BENCH-D fixtures through Kafka and S3, builds the analytics result,
waits for API readiness, and requires an unassociated service account's S3
probe to fail.

The imperative lab deployer replaces only the known fixture/analytics Jobs
before a redeploy because a Job pod template is immutable. Slice 4 will replace
this temporary lifecycle rule with explicit Argo CD hook or generated-run
semantics.

## 4. Teardown

Delete application Jobs/Deployments first, then Kafka custom resources while
the Strimzi operator is still running, wait for EBS volumes to disappear, and
only then destroy the OpenTofu lab. Inspect for orphaned load balancers,
volumes, snapshots, network interfaces, and ECR/S3 content.

`force_destroy_data=false` is the safe default. For a verified synthetic-only
lab, explicitly setting it to `true` allows versioned S3 objects and ECR images
to be removed during teardown. The state foundation is not part of normal lab
teardown.

## Static verification

```bash
TOFU_BIN=tofu scripts/ci/validate-aws-platform.sh
```

This validates both OpenTofu roots against the pinned AWS provider, runs mocked
offline infrastructure plans, renders all AWS Kustomize overlays, rejects
static AWS credential references, and asserts the three-zone, Kafka durability,
registry, state, and Pod Identity service-account boundaries.

## Authoritative references

- [EKS Pod Identity](https://docs.aws.amazon.com/eks/latest/userguide/pod-identities.html)
- [EKS managed node groups](https://docs.aws.amazon.com/eks/latest/userguide/managed-node-groups.html)
- [EKS EBS CSI driver](https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html)
- [EKS version and platform support](https://docs.aws.amazon.com/eks/latest/userguide/platform-versions.html)
- [OpenTofu S3 backend and native locking](https://opentofu.org/docs/language/settings/backends/s3/)
- [Strimzi deployment and configuration](https://strimzi.io/docs/operators/latest/deploying)
- [AWS Budgets behavior](https://docs.aws.amazon.com/cost-management/latest/userguide/budgets-managing-costs.html)
