# Slice 2 Verification Record

| Field | Value |
| --- | --- |
| Date | 2026-07-21 |
| Branch | `feature/market-events-to-object-storage` |
| Status | Runtime contract and scoped lifecycle passed; Slice 2 complete |
| Final state | Data path running on context `kind-investment-platform` |

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

The four keys were the deterministic QQQ, QQQM, FSELX, and SP500 objects under:

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
`synthetic:price:fselx:ack-unknown-001` immediately before the injected failure.
The retry then sent the same stable event ID, resulting in two Kafka records but
one FSELX archive object.

For the consumer boundary, the verifier inserted a 30-second delay after the
SP500 S3 write, observed the `post-write failure window open` marker, deleted that
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
`148.59s` and reached the same 7/1/4 record state with QQQ, QQQM, FSELX, and
SP500 as the four archived objects.

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

## Evidence handling

The final diagnostic bundle was written to:

```text
${TMPDIR}/market-data-path-diagnostics-20260721T233934Z
```

The diagnostic script excludes Secret objects, Secret values, archive object
contents, kubeconfigs, and credentials. Synthetic event IDs and deterministic
object keys are safe test fixtures.

## Conclusion

Slice 2 proves the first canonical producer-to-storage vertical path with an
explicit schema, per-workload mutual-TLS identities and ACLs, at-least-once
delivery, idempotent raw archive effects, malformed-record quarantine, producer
acknowledgement uncertainty, consumer crash recovery, and scoped teardown. Raw
object backup/restore, node failure, multi-replica stateful availability,
metrics/traces/alerts, schema registry, and cloud workload identity remain later
slices.
