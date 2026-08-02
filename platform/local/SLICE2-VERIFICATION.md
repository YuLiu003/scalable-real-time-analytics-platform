# Slice 2 Verification Record

| Field | Value |
| --- | --- |
| Date | Historical run 2026-07-21; current revalidation 2026-07-30 |
| Historical branch | `feature/market-events-to-object-storage` |
| Current revalidation | `feature/kafka-scale-lab` working tree based on `5ae7dd3` |
| Status | Current disposable runtime revalidation passed |
| Final state | kind and Colima deleted after successful verification |

> Evidence integrity notice: this record was captured before the public
> fixtures and operational logs were anonymized. Identifiers in this copy are
> fictional replacements, not the literal values observed on 2026-07-21.
> Treat the detailed narrative as historical design context. The current
> revalidation below uses only fictional public fixtures.

## Current revalidation

`make -C platform/local e2e-ephemeral` reproduced 7 market records, 1
quarantine record, and 4 immutable baseline objects. The acknowledgement retry,
post-write consumer crash, idempotent duplicate recovery, and exact terminal
counts passed. Operational logs used one-way event references, archive
inspection emitted counts without keys, and the disposable kind cluster and
Colima data were deleted afterward.

## Tested boundary

The test covers one synthetic producer, Strimzi Kafka `4.2.1`, one raw-event
archiver, and Garage `v2.3.0` on the Slice 1 kind cluster. Kafka and Garage each
have one replica. This is evidence for API integration, authentication,
persistence, delivery semantics, and recovery from process/pod failure; it is
not evidence of broker, storage, node, or zone availability.

The declared behavior is documented in the
[`Slice 2 event contract`](../../docs/features/cloud-native-investment-platform/slice-2-event-contract.md).

## Static verification

The following checks passed:

```bash
bash -n platform/local/scripts/*.sh
jq empty contracts/events/market.price.observed.v1.schema.json
kubectl kustomize platform/gitops/platform/local/market-data-services
kubectl kustomize platform/gitops/apps/local/market-pipeline
GOWORK=off go test ./...
git diff --check
```

Both Kustomize trees also passed server-side dry-run against Kubernetes `v1.33.7`
with the Strimzi `kafka.strimzi.io/v1` CRDs installed. The market-pipeline module
used Go `1.25.12`; its event validation and archive hash/key tests passed.

## Runtime acceptance evidence

`make -C platform/local bootstrap-data-path` completed the success and failure
fixtures and asserted this exact settled state:

```text
topic=market.prices records=7 partitions=3
topic=ingestion.quarantine records=1 partitions=1
bronze object count=4
```

The four keys were the deterministic DEMO-ASSET-A, DEMO-ASSET-B, DEMO-ASSET-C, and DEMO-BENCH-D objects under:

```text
bronze/market.price.observed/v1/date=2026-07-21/source=synthetic/
```

Observed controller and workload state:

- Kafka, its single dual-role node pool, and both KafkaTopics were Ready.
- Both mutual-TLS KafkaUsers were Ready with simple ACL authorization.
- Garage and Kafka each had a bound persistent volume and a Ready pod.
- The raw-event archiver recovered to `1/1` with zero restarts on the replacement
  pod.
- The baseline, acknowledgement retry, and consumer-crash Jobs completed.
- The acknowledgement-unknown Job failed by design after its send was broker
  acknowledged.

The producer failure log recorded the broker acknowledgement for
`synthetic:price:demo-asset-c:ack-unknown-001` immediately before the injected failure.
The retry then sent the same stable event ID, resulting in two Kafka records but
one DEMO-ASSET-C archive object.

For the consumer boundary, the verifier inserted a 30-second delay after the
DEMO-BENCH-D S3 write, observed the `post-write failure window open` marker, deleted that
archiver pod, and waited for its replacement to log the same event with
`result=duplicate`. Only then did verification restore the normal Deployment.
This demonstrates the intended post-effect/pre-offset redelivery outcome without
claiming end-to-end exactly-once processing.

The malformed fourth baseline record produced exactly one record on
`ingestion.quarantine` and no archive object. The archive inspector proved that
the exact baseline duplicate and both failure-boundary retries did not create
additional objects.

## Lifecycle and interruption evidence

The Slice 2-only lifecycle was tested without deleting the kind cluster or its
monitoring stack:

```bash
CONFIRM_DESTROY_DATA_PATH=market-data-path \
  make -C platform/local destroy-data-path
make -C platform/local bootstrap-data-path
```

Teardown removed the Jobs, Deployment, Kafka users/topics/cluster, Garage,
generated local Secrets, public broker-CA ConfigMap, and both data volumes. It
also waits for selected Kafka, Strimzi, Garage, and PVC resources before claiming
success.

The subsequent clean rebuild was interrupted after the fresh stateful half was
Ready but before any application resources existed. Re-running the same
bootstrap reused the generated credentials and volumes, created the application
half, repeated all failure tests, and reached the same exact 7/1/4 record state
in `46.60s`. This is useful evidence that bootstrap resumes from a partial,
consistent checkpoint.

During the long interruption the local Strimzi operator lost its Kubernetes
leader lease while Kafka admin and API operations timed out, exited with code 1,
and restarted. It was not OOM-killed; Kafka and Garage remained Ready with zero
restarts. This is consistent with a laptop-hosted control plane becoming
unavailable and is not treated as cloud availability evidence.

The later fund-fixture refresh performed another complete scoped rebuild in
`148.59s` and reached the same 7/1/4 record state with DEMO-ASSET-A, DEMO-ASSET-B, DEMO-ASSET-C, and
DEMO-BENCH-D as the four archived objects.

## Failures found and durable corrections

1. Cross-namespace User Operator reconciliation initially failed with RBAC 403.
   Desired state now grants the Strimzi cluster operator its standard namespaced
   and entity-operator delegation roles only in `analytics-apps`.
2. Strimzi requires an explicit feature flag for an Entity Operator watched
   namespace. The pinned Helm values now enable that supported boundary.
3. KafkaUser Secrets contain the clients CA, while the broker endpoint uses the
   cluster CA. Bootstrap now publishes only the public broker CA as an
   application ConfigMap; private client keys remain in their generated Secrets.
4. Mounting that public CA over a file from another projected volume failed at
   container creation. The identity Secret and broker trust bundle now use
   separate read-only directories.
5. Initial teardown returned while Kafka dependents were still terminating. The
   destroy script now waits for the selected pods and PVCs before reporting
   completion.
6. A later scoped teardown deleted the Topic Operator concurrently with two
   terminating KafkaTopics, orphaning their `strimzi.io/topic-operator`
   finalizers. After preserving the deletion evidence, the already-deleting
   synthetic topics were released. The script now deletes and waits for topics
   while Kafka and its Topic Operator are still available.
7. The consumer-crash verifier originally accepted any event's post-write delay
   marker before deleting the archiver. A queued earlier event could therefore
   satisfy the wait before DEMO-BENCH-D was archived, making the recovered consumer
   correctly report `created` instead of the expected replay `duplicate`. Both
   waits now match the exact DEMO-BENCH-D event and durable duplicate log record.

## Evidence handling

The final diagnostic bundle was written to:

```text
${TMPDIR}/market-data-path-diagnostics-20260721T233934Z
```

The current diagnostic script excludes Secret objects, Secret values, event
IDs, archive keys and contents, kubeconfigs, and credentials.

## Conclusion

The current revalidation and historical Slice 2 run cover the canonical
producer-to-storage path,
mutual-TLS identities and ACLs, at-least-once delivery, idempotent archive
effects, quarantine, acknowledgement uncertainty, crash recovery, and scoped
teardown. Raw backup/restore, node failure, multi-replica stateful availability,
traces/alerts, schema registry, and cloud workload identity remain later
slices.
