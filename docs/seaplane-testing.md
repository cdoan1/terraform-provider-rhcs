# Seaplane (V2 HCP) E2E Testing — Coverage Inventory and Plan

**Status:** Draft for review — planning document, not implemented unless agreed
**Scope:** `tests/e2e/seaplane/` (V2) versus `tests/e2e/` (V1) e2e coverage, and the plan to close the structural and feature gaps
**Related:** ROSAENG-68171 (this work), parent ROSA-1174; feature epics ROSAENG-65173 / 65243 / 65120 / 65204

---

## 1. How the two suites run today

| | V1 (`tests/e2e/`) | V2 (`tests/e2e/seaplane/`) |
|---|---|---|
| Cluster model | **Shared cluster**: `BeforeSuite` resolves a cluster via `clusterworkflow.NewFetcher`; a `day1-prepare` spec creates it from the profile; day-2 specs mutate it; a `destroy` spec tears it down | **Self-contained specs**: every spec creates its own VPC → OIDC → IAM → cluster via `clusterworkflow.New` and destroys it in `DeferCleanup` (proxy is the one retained-cluster exception) |
| Spec selection | Labels (`day1`, `day1-prepare`, `day1-negative`, `day2`, `day3`, `destroy`, feature labels) selected by CI through `--label-filter` | Profile gating (`Skip` unless the profile enables the feature); one profile per CI job |
| Ordering | Enforced by **CI job sequencing** (day-1 job → day-2 jobs → destroy job), not by ginkgo | None — and none needed today: no spec depends on another |
| Entry point | `make e2e_test LabelFilter=...` (5h, `-r`, `--focus-file tests/e2e/.*`) | `make e2e-seaplane` (3h, no label filter) |
| Cluster cost per job | 1 cluster amortized across the phase set | **1 cluster per spec** (e.g. `seaplane-custom-cidrs` job = sanity's cluster + networking's cluster = 2 clusters) |

Notes:
- `make e2e_test`'s `--focus-file tests/e2e/.*` also matches the seaplane package, and `clusterworkflow.NewFetcher`/`NewDestroyer` dispatch on cluster type (SEAPLANE is supported) — but the V1 spec *bodies* are OCM-based (`cms` services) and only 5 of 25 V1 test files are cluster-type aware, so pointing `make e2e_test` at a seaplane profile fails outside a strict label filter.
- `make e2e_test LabelFilter="seaplane-validated"` does work as a selection (ginkgo skips suites whose specs are all filtered out), but `sanity` and `proxy` carry no label today and would be excluded.

---

## 2. Current V2 spec inventory (11 specs)

| File | Spec | Gate / cluster model | Feature (JIRA) |
|---|---|---|---|
| sanity_test.go | creates and destroys a seaplane cluster end-to-end | none — always runs; own cluster | core flow (ROSAENG-66461) |
| sanity_test.go | (within spec) 2 nodepools, replica scale, cluster tags verify | — | core flow |
| cluster_day1_test.go | creates a cluster with a scheduler profile | profile `scheduler_profile`; own cluster | ROSAENG-68167 |
| cluster_day1_test.go | configures autoscaler during cluster creation | profile autoscaler fields; own cluster | ROSAENG-65114 |
| cluster_day2_test.go | updates autoscaler and component routes as a day-2 operation | profile; own cluster | ROSAENG-65114 |
| cluster_capabilities_test.go | creates a cluster with the ImageRegistry capability disabled | profile `seaplane-disable-caps` (pinned by the spec); own cluster | ROSAENG-65243 |
| cluster_networking_test.go | creates a cluster with custom machine/pod/service CIDRs | profile CIDR fields; own cluster | ROSAENG-65120 |
| nodepool_tags_test.go | creates a nodepool with tags and updates them in place | profile `tagging`; own cluster | ROSAENG-65173 |
| nodepool_tags_test.go | rejects invalid nodepool tags during validation | none (client-side; no cluster needed) | ROSAENG-65173 |
| nodepool_gpu_test.go | creates a nodepool with a GPU instance type | opt-in env `SEAPLANE_NODEPOOL_GPU_INSTANCE_TYPE` | ROSAENG-65204 |
| nodepool_autoscaling_test.go | creates a nodepool with autoscaling, updates min/max | profile `autoscaling_enabled`; own cluster | ROSAENG-65304 |
| proxy_test.go | creates, updates, clears proxy settings | **retained cluster** via `SEAPLANE_CLUSTER_NAME` (skips when unset) | ROSAENG-68198 |

---

## 3. V1 → V2 coverage comparison

Status legend: **Covered** (V2 spec exists) · **Partial** (some coverage, gaps remain) · **Gap** (applicable, no V2 spec) · **N/A** (OCM-only concept, never applies to V2) · **Investigate** (applicability unclear)

### 3.1 Cluster lifecycle

| V1 spec (file: id) | V1 phase | V2 status | Notes |
|---|---|---|---|
| cluster_creation: CreateClusterByProfile | day1-prepare | **Gap (planned)** | Shared-cluster prepare spec is part of the phase-model plan; sanity covers create e2e |
| cluster_creation: 66071 (recreate after out-of-band delete) | day3 | **Gap** | Applies to V2 (Platform API delete + re-apply) |
| cluster_creation: 74472 / 74473 / 74748 (waiter behaviors) | day1-supp | **Partial** | `CreateAndWaitReady` used by every spec; error-path and no-wait variants untested |
| cluster_creation: 72500 (destroy without waiting) | day3 | **Gap** | |
| cluster_destroy: DestroyClusterByProfile | destroy | **Gap (planned)** | Destroyer exists (used in DeferCleanup); no standalone destroy phase |
| verification_post_day1: 74096 (resources wait for ready) | day1-post | **Covered** | `CreateAndWaitReady` + sanity |

### 3.2 Day-1 create-time verification (day1-post family)

| V1 spec (id) | V2 status | Notes |
|---|---|---|
| 63134 successfully installed | **Covered** | Ready-phase assertion in every profile spec |
| 88408 autoscaling correctly set | **Covered** | cluster_day1 autoscaler spec |
| 67607 proxy correctly set | **Covered** | proxy spec verifies via Platform API |
| 63777 AWS tags are set | **Covered** | sanity asserts `spec.tags` (create-time; V2 tags are immutable) |
| 72466 host prefix and CIDRs set correctly | **Covered** | cluster_networking spec (ROSAENG-65120) |
| 76500 registry config set correctly | **Covered (different mechanism)** | V2 analog = capabilities spec: ImageRegistry disable (ROSAENG-65243) |
| 64906 custom properties | **Gap** | `spec.properties` passthrough exposed in pathbind |
| 63133 private_link enabled/disabled | **Gap** | `endpoint_access` passthrough exposed (private cluster) |
| 75107 multiarch | **Gap** | `multi_arch` exposed in the aws bundle |
| 63141 AZs and multi-az | **Partial** | V2 e2e is single-AZ only |
| 69145 / 77065 additional security groups | **Gap** | nodepool `additional_security_group_ids` (provider support landed in ROSAENG-68136) |
| 69143 worker disk size | **Gap** | nodepool `rootVolume` passthrough |
| 64023 / 73153 compute machine type / replicas | **N/A** | No default machine pool in V2 — explicit nodepools (covered by nodepool specs) |
| 68423 compute labels | **N/A** | Default machine pool concept |
| 63140 fips | **N/A** | service-set in V2 |
| 63143 / 72485 etcd encryption / KMS | **N/A** | service-set |
| 63334 customer-managed KMS | **N/A** | service-set |
| 63950 / 75372 imdsv2 | **N/A** | Not exposed in the V2 pathbind draft |
| 65928 admin user | **N/A** | OCM-only |
| 63138 account roles unified path | **N/A** | V2 uses user-managed IAM (aws manifest) |

### 3.3 Day-2 cluster edits

| V1 spec (id) | V2 status | Notes |
|---|---|---|
| 72489 / 72490 proxy (apply / edit fields) | **Covered** | proxy spec: apply, update, clear; negative gated by env |
| 63147 autoscaling change | **Covered** | cluster_day2 spec |
| 69137 / 72524-72526 cluster autoscaler lifecycle | **Covered** | day-1 config + day-2 update specs |
| 72628 tags edit | **Covered (divergent semantics)** | V2 cluster tags are immutable — replacement covered in subsystem; e2e covers create+verify |
| 72470 network fields (CIDR immutability) | **Covered** | Immutable in V2; replacement asserted in subsystem; e2e covers create+verify |
| 72517 / 72520 ingress edit (component routes) | **Covered** | cluster_day2 component routes update |
| 72452 / 72449 / 72478 / 72486 / 72496 / 75414 required/compute/version/encryption/sts/imdsv2 fields | **N/A** | OCM-side validation or service-set fields |
| 72455 / 72451 / 64907 properties | **Gap** | `spec.properties` exposed |
| 72480 private fields | **Gap** | `endpoint_access` |
| 69145 security groups | **Gap** | nodepool-level in V2 |

### 3.4 Machine pools / node pools

| V1 spec (id) | V2 status | Notes |
|---|---|---|
| 72504 create with required attributes | **Covered** | sanity nodepools |
| 72505 / 68296 autoscaling create/edit | **Covered** | nodepool_autoscaling spec |
| 72510 (HCP) / 73942 (classic) tags | **Covered** | nodepool_tags spec; V2 divergence: mutable in-place (documented) |
| 64757 second machine pool | **Covered** | sanity np1/np2 |
| 72954 multiple instances | **Covered** | sanity (2 pools) + replica scale |
| "updates fixed replicas through the selected backend" | **Covered** | sanity scales np2 (spec is dual-labeled `seaplane-validated`) |
| labels create (72514 partial) | **Partial** | labels set at create in sanity; edit spec missing |
| 72514 / 73431 creation / edit fields | **Partial** | autoscaling + replicas covered; labels/taints/autorepair edits missing |
| 65645 reconcile basic flow | **Partial** | drift-free round-trips asserted in subsystem; e2e plan-no-change missing |
| 68283 / 63139 creation validation | **Partial** | provider-side + subsystem validation; e2e negative only for tags |
| 66403 / 72960 import | **Gap** | `ImportState` implemented, e2e-untested (known gap in developer-docs) |
| 64905 labels edit/delete | **Gap** | |
| 64904 taints | **Gap** | `taints` passthrough exposed |
| 69146 / 73068 / 73069 additional security groups | **Gap** | exposed (ROSAENG-68136) |
| 69144 / 76345 disk size | **Gap** | `rootVolume` |
| 72508 / 72521 / 72522 tuning configs | **Gap** | `tuningConfig` passthrough exposed |
| 74520 / 70128 / 70129 kubelet config | **Gap** | kubelet passthrough exposed |
| 87302 image type | **Gap** | `imageType` passthrough exposed |
| node_drain_grace_period | **Gap** | `nodeDrainTimeout` passthrough exposed |
| 72509 specific version | **Gap** | nodepool `release` image passthrough exists; version-pinning semantics differ |
| 65063 / 65071 single-AZ pool for multi-AZ cluster / subnet option | **Gap** | multi-AZ untested in V2 |
| 69009 / 69727 / 69073 default machine pool lifecycle | **N/A** | No default pool in V2 |
| 72512 / 72513 y-1 / z-1 nodepool upgrades | **N/A** | OCM upgrade pipeline |
| 75373 / 75393 imdsv2 | **N/A** | Not exposed in V2 draft |

### 3.5 Negative / validation (day-1)

| V1 spec (id) | V2 status | Notes |
|---|---|---|
| 72627 tags fields | **Covered** | provider validation + subsystem + e2e negative spec |
| 72468 network fields | **Covered** | provider validation (parse/overlap, V1 wording parity) + subsystem; V2 API rejects the rest |
| 72491 proxy fields | **Covered** | over-limit negative in proxy spec (env-gated) |
| 72445 required fields | **Partial** | provider subsystem validation; no dedicated e2e |
| 88409 autoscaling setting | **Partial** | nodepool autoscaling validation in handler + subsystem |
| 72449 / 72477 / 72486 / 72496 / 75392 / 64095 / 65961 / 65963 compute/version/encryption/sts/imdsv2/username/password | **N/A** | OCM-side validation or N/A concepts |
| 76344 worker disk size | **Gap** | |
| 63825 BYOVPC and region match | **Gap** | VPC-CIDR ⊆ machine-CIDR containment is V2-relevant (documented constraint, not asserted) |

### 3.6 Other resources

| V1 spec (id) | V2 status | Notes |
|---|---|---|
| 85748 / 85908 break glass credential | **N/A** | OSD/OCM concept |
| 65380 / 63316 / 67574 account roles | **N/A** | `rhcs_account_roles` is an OCM resource; V2 uses the aws IAM manifest |
| idps family (63151, 73154, …) | **N/A** | OCM IDP resources |
| external-auth-01 / 02 external auth | **Investigate → Gap** | `spec.hostedCluster.configuration.authentication.oidcProviders` passthrough exposed; shape of TF support TBD |
| 67570 dns domain | **N/A** | OCM DNS management |
| image mirror family | **Investigate → Gap** | `imageContentSources` passthrough exposed |
| 68301 info data source | **N/A** | OCM data source |
| 75233 trusted IPs | **Investigate → Gap** | `networking.apiServer.allowedCIDRBlocks` exposed |
| trust policy external ID | **N/A** | OCM STS |
| 63152 / 63153 y/z-stream upgrades | **Investigate** | `ControlPlaneUpgradePolicy` types exist in the API; not exposed in TF yet |
| classic_ingress family (70336, 73610, 75067, 70337) | **N/A** | Classic-only LB configuration |

### 3.7 Summary counts

| Status | V1 specs (approx.) | Meaning |
|---|---|---|
| Covered | ~25 | V2 equivalent exists (sometimes with divergent, documented semantics) |
| Partial | ~12 | Core flow covered; edit/negative variants missing |
| Gap | ~30 | Applicable and exposed in the V2 pathbind, but no spec |
| N/A | ~80 | OCM-only concepts (IDPs, account roles, DNS, upgrades, service-set fields, …) |
| Investigate | ~6 | Need a decision before planning (external auth, image mirror, allowed CIDRs, upgrade policies) |

---

## 4. Plan

### Phase 1 — mechanical unification (small, no behavior change)
1. Label `sanity` and `proxy` specs with `ci.SeaplaneValidated` (proxy additionally with a phase label later).
2. Add `--label-filter $(LabelFilter)` passthrough to `make e2e-seaplane`, mirroring `make e2e_test`.
3. Add `terraform validate` over `tests/tf-manifests/**` to pre-push checks (the undeclared-variable incident was only caught mid-e2e).

### Phase 2 — shared-cluster phase model (structural)
Adopt the V1 execution model for V2, keeping per-spec clusters only where create-time settings demand it:
- Add a `day1-prepare` spec (creates the shared cluster from the profile) and a `destroy` phase spec; `BeforeSuite` resolves the cluster via the existing `seaplaneFetcher`.
- Convert **day-2 specs** (nodepool tags, autoscaling, proxy, component routes) to run against the shared cluster.
- Keep **create-time feature specs** (capabilities, CIDRs, scheduler profile) on their own clusters — they are immutable create-only settings and cannot share a cluster.
- The proxy spec's retained-cluster pattern dissolves into the model (it becomes a normal day-2 spec).
- Benefit: one cluster per profile job instead of N; CI can drive phases with `make e2e-seaplane LabelFilter="day2"`.

### Phase 3 — feature backlog (from §3 gaps, prioritized)
- **P1 nodepool day-2 family:** labels/taints/autorepair edits, additional security groups, disk size, tuning configs, kubelet config, image type, import, node drain timeout
- **P2 cluster:** properties, endpoint access (private), multiarch, multi-AZ, BYOVPC/CIDR containment negative, recreate-after-out-of-band-delete, waiter error paths
- **P3 investigations:** external auth (oidcProviders), image mirror (imageContentSources), allowed CIDR blocks, control-plane upgrade policies

---

## 5. Open questions

1. Should external auth / image mirror / allowed CIDR blocks be exposed as friendly TF attributes (like the network CIDRs) or stay raw JSON passthrough?
2. Upgrade policies: in scope for the TF provider at all in EP1, or deferred?
3. Phase 2 ordering: do we want ginkgo `Ordered` containers for in-run grouping, or strictly CI-job-driven phases like V1? (V1 convention is CI-driven.)
4. Does the GPU spec stay opt-in (env var) or get a dedicated CI profile once G-instance quota is confirmed?
