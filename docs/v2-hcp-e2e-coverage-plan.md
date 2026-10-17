# TF E2E Coverage Plan — Seaplane V2 HCP Flow

**Epics:** ROSAENG-65243 (Image Registry Enable/Disable), ROSAENG-65173 (Custom Node Pool Tags), ROSAENG-65120 (Custom Network CIDRs), ROSAENG-65204 (GPU Machine Pools)
**Parent:** ROSA-1174 — Seaplane API passthrough features for Engineering Preview 1
**Status:** In progress — see "Implementation status" below

---

## 0. Implementation status

| Epic | Provider | Unit | Subsystem | E2E | Status |
|------|----------|------|-----------|-----|--------|
| ROSAENG-65173 node pool tags | ✅ `tags` map + validation + encode/flatten in `nodepool_handler.go` | ✅ | ✅ 5 specs (create/update/round-trip/negatives) | ✅ `nodepool_tags_test.go` (gated on `tagging: true` profile) | **Code complete — pending e2e run** |
| ROSAENG-65204 GPU machine pools | n/a (no provider change needed) | n/a | n/a | ✅ `nodepool_gpu_test.go` (opt-in via `SEAPLANE_NODEPOOL_GPU_INSTANCE_TYPE`) | **Code complete — pending e2e run** |
| ROSAENG-65243 capabilities | ✅ `enabled/disabled_capabilities` JSON-string attributes (pathbind passthrough — note: SDK stores `[]OptionalCapability`, a named type pathbind cannot convert from `[]string`, so the repo's JSON-string convention for arrays applies, same as `componentRoutes`/`resource_tags`) + whitelist/overlap validation | ✅ (via subsystem) | ✅ 4 specs (create body, replacement plan, invalid capability, enabled+disabled overlap) | ✅ `cluster_capabilities_test.go` + profile `seaplane-disable-caps` (pinned by the spec) | **e2e validated 2026-10-17** (day1 filter, ~18m: create → Ready → TF output + Platform API assertions → clean teardown) |
| ROSAENG-65120 network CIDRs | ✅ top-level `machine_cidr`/`pod_cidr`/`service_cidr` (plain CIDR strings) + `host_prefix` (V1 parity), consumer-only with handler encoding into the networking arrays; validation: CIDR parse, pairwise overlaps (V1 38857 wording parity), host-prefix and pod-prefix, mutual exclusion with the raw `network` bundle | ✅ (via subsystem) | ✅ 7 specs (create body, drift-free round-trip, replacement plan, invalid CIDR, overlap, host-prefix rules, bundle conflict) | ✅ `cluster_networking_test.go` + profile `seaplane-custom-cidrs` | **Code complete — pending e2e run** |

Additional fixes landed along the way:
- Restored the `rhcs_oidcconfig_seaplane` `name` attribute to `required`/client-selected (a regression from commit 1dad70e7 had reverted the override to `computed`, breaking 2 subsystem specs at HEAD).
- Fixed a stale `NodePools(clusterID)` call in `nodepool_autoscaling_test.go` (the SDK identity-flip removed the namespace argument).
- Regenerated `provider/seaplane/generated` against the updated `../rosa-seaplane-api` (branch `ROSAENG-68177-mirror-capabilities`) after `go mod tidy` + `go mod vendor`.

All 35 seaplane subsystem specs and provider unit tests pass locally, and `make pre-push-checks` is green through every step the local environment can run. `make docs` regenerated `docs/resources/{cluster,nodepool,oidcconfig}_seaplane.md` for the schema changes. Two notes:
- The seaplane schema descriptions in the generated docs are terse (e.g. `Machine_cidr.`) — the upstream pathbind-gen TF template hardcodes `{{.GoName}}.` and only surfaces override descriptions for list(object) element attributes. The rich descriptions live in `pathbind-overrides.yaml`; surfacing them in the schema needs an upstream pathbind-gen change (follow-up).
- `make docs-lint` (vale) fails locally only because it scans the gitignored `vendor/` directory plus one pre-existing `.vale.ini` self-flag; CI containers do not vendor and the non-vendor errors (3 `vs` wording hits in `developer-docs/seaplane.md`) were fixed along the way.

E2E still requires the real V2 environment: `SEAPLANE_URL`, `SEAPLANE_RELEASE_IMAGE`, `CLUSTER_PROFILE` (e.g. `seaplane-basic` for the tags test; the capabilities and CIDR specs pin their own profiles), and AWS credentials.

---

All four epics are `complexity:passthrough` capabilities under ROSA-1174. Three are labeled `ROSASeaplane:APIv1` (feature exists in V1, needs V2 coverage); image registry is `ROSASeaplane:APIv2`.

| Epic | Feature | V2 (Seaplane) API path | Mutability | rosa CLI (V2) surface |
|------|---------|--------------------------|------------|------------------------|
| ROSAENG-65243 | Image Registry Enable/Disable | `spec.hostedCluster.capabilities.{enabled,disabled}` — `ImageRegistry` is one of 7 valid capabilities | Create-only, immutable | `--enable-cluster-capabilities` / `--disable-cluster-capabilities` (comma-separated; validated in `cmd/create/cluster/seaplane.go` `encodeSeaplaneClusterCapabilities`) |
| ROSAENG-65173 | Custom Node Pool Tags | `spec.nodePool.platform.aws.resourceTags` — `[]{key,value}` | Mutable (in-place) | `--tags` on machinepool create/edit (`cmd/create/machinepool/seaplane.go`, max 25, key≤128, value≤256, `aws:` prefix reserved) |
| ROSAENG-65120 | Custom Network CIDRs | `spec.hostedCluster.networking.{machineNetwork,clusterNetwork,serviceNetwork}` — array of entries; `hostPrefix` lives **inside** `clusterNetwork` entries | Create-only, immutable | `--machine-cidr`, `--pod-cidr`, `--service-cidr`, `--host-prefix` (encoded to JSON arrays in `cmd/create/cluster/seaplane.go` ~L360–385; host prefix defaults to 23) |
| ROSAENG-65204 | GPU Machine Pools | `spec.nodePool.platform.aws.instanceType` — **no instance-type restriction or validation in V2** | n/a (immutable per nodepool) | `--instance-type` passthrough; epic status: "we can already do that through the CLI/API/SDK. Just need to validate the behavior on TF side." |

### Related rosa CLI work items (V2)

- **Capabilities:** rosa CLI V2 e2e already validated — `tests/e2e/seaplane_capabilities_test.go` (ROSAENG-68175, profile `rosa-seaplane-disable-capabilities`): creates a cluster with `--disable-cluster-capabilities ImageRegistry` and asserts the describe JSON reports `capabilities.disabled: [ImageRegistry]` with no `enabled` key.
- **Node pool tags:** rosa CLI V1 e2e `[id:73638]` (tag validation rules — *already validated in V2*) and `[id:73492]` (managed/cluster/user tag composition — pending decision whether it duplicates 73638).
- **Network CIDRs:** rosa CLI V1 e2e `[id:38857]` (Day-1 negative network validation, Seaplane: InProgress) and `[id:70970]` (Day-1 negative, CIDR without pre-configured VPC, Seaplane: InProgress).
- **GPU:** no dedicated rosa CLI test; V2 does not restrict instance types.

---

## 1. Current state in the TF repo (gap analysis)

### 2.1 Provider support per feature

| Feature | Provider status today |
|---------|----------------------|
| Image registry capabilities | **Not exposed.** No `capabilities` fields in `provider/seaplane/pathbind-overrides.yaml` or the generated schema. (`image_registry_arn` is a different thing — the IAM rolesRef for the registry operator.) |
| Node pool tags | **Raw passthrough only.** Generated `resource_tags` (JSON string) on `rhcs_nodepool_seaplane`. The nodepool manifest has a `tags` variable that is **dead** — defined in `variables.tf` but never referenced in `main.tf`; `SeaplaneNodePoolArgs.Tags` exists but is unused. No validation, no user-friendly map. |
| Network CIDRs | **Generated but unwired.** The generated cluster schema has a `network` bundle (`machine_cidr`, `pod_cidr`, `service_cidr`, `network_type`, `api_allowed_cidr_blocks`) as JSON strings, but it is not referenced by the test manifest, the handler, `SeaplaneClusterArgs`, or any profile. There is **no `host_prefix`** field (hostPrefix is nested inside clusterNetwork entries in the API). |
| GPU machine pools | **Already supported.** `instance_type` is Optional on `rhcs_nodepool_seaplane` and wired through the nodepool manifest. No e2e coverage with GPU types. |

### 2.2 Existing V1 TF e2e tests to mirror

| Feature | V1 TF test | What it covers |
|---------|-----------|----------------|
| Node pool tags | `tests/e2e/hcp_machine_pool_test.go` — `[id:72510]` "can be created with tags" | Create MP with `tags` map, verify via OCM API; empty-tags case. Negative cases (same file): system tag `api.openshift.com/id` rejected; tags immutable on edit (`Attribute aws_node_pool.tags, cannot be changed from`) |
| Network CIDRs | `tests/e2e/negative_day_one_test.go` (~L390–420) | Negative: machine CIDR outside VPC range; machine/service CIDR overlap; machine/pod overlap; service/pod overlap; CIDR without consistent host prefix |
| Network CIDRs | `tests/e2e/cluster_edit_test.go` (~L378–398) | Immutability: `host_prefix`, `machine_cidr`, `service_cidr`, `pod_cidr` cannot be changed after create |
| GPU / instance types | `tests/e2e/hcp_machine_pool_test.go` machine-type cases; profile `compute_machine_type` (`id:72448`) | Instance type set at MP create; no GPU-specific case |
| Image registry | — | **No V1 analog.** OCM-side support (ROSA-141) is still To Do; TF V1 only exposes the image-registry operator IAM policy ARNs |

> Note: V1 HCP machinepool tags are **immutable**; the V2 Seaplane `resourceTags` field is **mutable** (HyperShift propagates in-place). The V2 test must assert in-place update instead of the V1 immutability assertion.

### 2.3 Established V2 enablement pattern (recent commits)

Reference commits, in increasing scope:

- **ROSAENG-65293** (444af689) — cluster custom AWS tags: pathbind `spec.tags` as map + handler validation (`validateClusterTags`) + manifest/args/profile wiring + sanity e2e addition + subsystem context.
- **ROSAENG-68167** (04cb06d2) — scheduler profile: pathbind alias + regen + manifest vars + `cluster_configuration_test.go` (now `cluster_day1_test.go`) profile-gated e2e.
- **ROSAENG-65304** (5300976c) — nodepool autoscaling: pathbind consumer-only bundle + nodepool handler + manifest + `nodepool_autoscaling_test.go` profile-gated e2e + subsystem.
- **ROSAENG-68198** (fd65db9a) — proxy: handler + manifest + `proxy_test.go` retained-cluster e2e (positive, update, clear + optional negative via env gate).

Common anatomy of a feature PR:

1. `provider/seaplane/pathbind-overrides.yaml` — field mapping (alias, type, immutable, description)
2. `make generate-seaplane` → `provider/seaplane/generated/*` (uses upstream rosa-seaplane-api main draft)
3. `provider/seaplane/{cluster,nodepool}_handler.go` — validation + encode/decode
4. `tests/tf-manifests/rhcs/{clusters,node-pools}/seaplane/{main.tf,variables.tf,output.tf}`
5. `tests/utils/exec/seaplane_{cluster,nodepool}.go` — Args/Output structs
6. `tests/utils/clusterworkflow/seaplane.go` — profile → args mapping
7. `tests/utils/profilehandler/{profile.go,handler.go}` — profile fields/getters
8. `tests/ci/profiles/tf_seaplane_cluster_profiles.yml` — CI profiles
9. `tests/e2e/seaplane/*_test.go` — e2e
10. `subsystem/seaplane/*_test.go` — stubbed Platform API tests (also keeps `make check-subsystem-registry` meaningful)
11. docs/examples regeneration

Two e2e styles are available:

- **Profile-gated full lifecycle** (`cluster_day1_test.go`, `nodepool_autoscaling_test.go`): `profilehandler` + `clusterworkflow.New(...)` creates a cluster from profile tfvars, `DeferCleanup` destroyer. **Use for create-only settings** (capabilities, CIDRs) and nodepool Day-2 features.
- **Retained cluster** (`proxy_test.go`, `sanity_test.go`): reuses `SEAPLANE_CLUSTER_NAME` workspace. Only suitable for mutable settings.

---

## 2. Per-feature implementation plan

### 3.1 ROSAENG-65173 — Custom Node Pool Tags (start here; self-contained Day-2)

**UX decision:** expose a user-friendly `tags` map on `rhcs_nodepool_seaplane` (parity with the cluster resource's `tags` and V1 `aws_node_pool.tags`), encoded by the handler to `spec.nodePool.platform.aws.resourceTags` JSON. Keep the raw generated `resource_tags` string as-is (passthrough escape hatch), or hide it if it complicates state round-trips.

**Provider:**
- Add consumer-only `tags` map alias in `pathbind-overrides.yaml` nodePool section (same mechanism as the autoscaling `enabled` bundle field).
- `nodepool_handler.go`:
  - `validateNodePoolTags` mirroring `validateClusterTags` + rosa CLI rules: max 25 tags, key ≤128 / value ≤256 chars, `aws:`/`red-hat` reserved prefixes, key/value charset regex (wordify the CLI's `ERR: expected a valid user tag key/value` / `invalid tag format` messages).
  - PreExpand/PostExpand: encode map → `[{"key":k,"value":v},…]` JSON for the native `ResourceTags` string; flatten back on read.
- Regenerate; update `docs/resources/nodepool_seaplane.md` + example.

**Test wiring:**
- `tests/tf-manifests/rhcs/node-pools/seaplane/main.tf`: reference the existing (currently dead) `tags` variable on the resource.
- `SeaplaneNodePoolArgs.Tags`: make it `*map[string]string` for consistency with other pointer fields; add tags to `SeaplaneNodePoolOutput` (echo through `output.tf`).

**E2E** — new `tests/e2e/seaplane/nodepool_tags_test.go`, mirroring V1 `[id:72510]` + rosa CLI `[id:73638]`/`[id:73492]` (profile-gated full-lifecycle style, like `nodepool_autoscaling_test.go`):
1. Create nodepool with 2 user tags (replicas 1, cheap instance type); verify Platform API `np.Spec.NodePool.Platform.AWS.ResourceTags` contains them; verify TF output.
2. **Update** tags (add/remove a key) → apply succeeds in-place; Platform API reflects new set (V2 mutable — diverges intentionally from V1 immutability).
3. Negative (client-side validation, no cluster needed — can also live in subsystem): too many tags (>25), overlong key/value, `aws:` prefix, invalid charset — each rejected by the provider with actionable errors.
4. Optional (API-side, if Platform API enforces): malformed tags rejected by the API.

**Subsystem** — `subsystem/seaplane/nodepool_resource_test.go`: context "NodePool tags" — create POST body asserts resourceTags JSON; update in place; validation diags; (no replacement since mutable).

**Estimated scope:** ~medium; follows 65293 almost exactly.

### 3.2 ROSAENG-65204 — GPU Machine Pools (e2e only; smallest)

No provider change required — `instance_type` already passes through and V2 does not validate instance types (confirmed against `cmd/create/machinepool/seaplane.go`; epic status agrees).

**E2E** — new `tests/e2e/seaplane/nodepool_gpu_test.go`:
1. Create a nodepool with a GPU instance type (default `g5.xlarge`; override via env `SEAPLANE_NODEPOOL_GPU_INSTANCE_TYPE` or profile `compute_machine_type`), replicas 1.
2. Wait for `NodePoolPhaseReady` via Platform API; assert `np.Spec.NodePool.Platform.AWS.InstanceType == "g5.xlarge"`.
3. Optional deeper check (cheap, no GPU workload): describe the backing EC2 instances via the existing AWS helpers (`tests/utils/helper/aws.go`) and confirm the instance type / GPU device presence.
4. Document (test comment) that V2 performs no instance-type name validation — an invalid type fails at AWS provisioning time and is out of scope.

**CI:** add a dedicated profile (e.g. `seaplane-gpu-machine-pool`) or gate the test behind an env var so the (expensive, quota-limited) GPU case does not run in every seaplane job. Confirm the CI account has `G` instances quota in `us-east-1`.

**Estimated scope:** small.

### 3.3 ROSAENG-65243 — Image Registry Enable/Disable (capabilities)

**Provider:**
- Confirm the upstream rosa-seaplane-api `main` pathbind draft carries `spec.hostedCluster.capabilities.{enabled,disabled}` (it does in the rosa CLI's vendored draft, `array(string)`, create-only; the local `../rosa-seaplane-api` replace is on a stale branch — regenerate via `make generate-seaplane` which clones main, or update the local checkout).
- Add overrides: aliases `enabled_capabilities` / `disabled_capabilities`, type string (comma-separated list, matching CLI UX; encoded to JSON array), `immutable: true`, `operations: [create]`.
- `cluster_handler.go`: `validateClusterCapabilities` mirroring the CLI's `encodeSeaplaneClusterCapabilities`:
  - whitelist: `ImageRegistry, OpenShiftSamples, Insights, Baremetal, Console, NodeTuning, Ingress`;
  - reject a capability present in both lists (HyperShift CEL: "Capabilities can not be both enabled and disabled at once");
  - encode comma-separated input → JSON array in PreExpand; flatten on read.
- Wire manifests (`variables.tf` + `main.tf`), `SeaplaneClusterArgs` (+Output), `clusterworkflow`, profilehandler (`disable_cluster_capabilities` / `enable_cluster_capabilities` profile fields).
- Regenerate; docs/examples.

**E2E** — new `tests/e2e/seaplane/cluster_capabilities_test.go`, mirroring rosa CLI ROSAENG-68175 (profile-gated full-lifecycle style):
1. Profile `seaplane-disable-caps` (pinned by the spec) with `disable_cluster_capabilities: ["ImageRegistry"]`.
2. Create cluster via `clusterworkflow` backend; assert TF output echoes the setting.
3. Verify via Platform API: `cluster.Spec.HostedCluster.Capabilities.Disabled` contains `ImageRegistry` and `Enabled` is empty/absent.
4. Verify cluster reaches `Ready` with the capability disabled.
5. Negative (provider-side, subsystem/e2e): invalid capability name rejected with the valid-values list; enabled+disabled overlap rejected.
6. Immutability: changing `disabled_capabilities` after create must plan a replacement (subsystem plan assertion).

**Subsystem** — `subsystem/seaplane/cluster_resource_test.go`: context "Cluster capabilities" — POST body assertion, validation diags, replacement-on-change plan (pattern from the 65293 tags context).

**Scope note (from JIRA comments):** the ephemeral test environment's registry is enabled-by-default but not backed by storage (`image-registry` CO Degraded, `StorageExists=False`). The TF test validates the *capability setting* and cluster readiness — not registry pod health — matching the rosa CLI test's scope.

**Estimated scope:** medium.

### 3.4 ROSAENG-65120 — Custom Network CIDRs (largest; do last)

**UX decision:** V1 TF and the rosa CLI expose `machine_cidr`, `pod_cidr`, `service_cidr`, `host_prefix` as plain values. The generated `network` bundle exposes raw JSON strings (`[{"cidr":"…"}]`) because the draft types are `array(object)`. Recommended: user-friendly consumer-only attributes `machine_cidr`, `service_cidr`, `pod_cidr` (CIDR strings) and `host_prefix` (int64), encoded by the cluster handler into the networking arrays — exactly mirroring the CLI bridge (`cmd/create/cluster/seaplane.go` ~L360–385: `machineNetwork=[{cidr}]`, `clusterNetwork=[{cidr, hostPrefix}]` with host prefix defaulting to 23, `serviceNetwork=[{cidr}]`).

- Alternative (rejected): wire the generated `network` bundle JSON strings directly — poor UX, diverges from V1 TF and CLI.
- The generated bundle can stay for raw passthrough, with the friendly fields taking precedence (or the bundle attributes hidden) — decide during implementation to keep state round-trips unambiguous.

**Provider:**
- Consumer-only aliases in pathbind overrides + regen.
- `cluster_handler.go`:
  - Encode: CIDR strings + host prefix → networking JSON arrays (create only).
  - Flatten: parse arrays back into the friendly attributes on read.
  - `validateClusterNetwork` (V1 `[id:38857]` parity, provider-side): CIDR parse errors; block-size checks (single-AZ machine CIDR `/16`–`/25`); service/pod CIDR "range too small" checks; pairwise overlap checks (machine/service, machine/pod, service/pod); host prefix 23–26; consistency of host prefix with pod CIDR.
- Wire manifests/args/output, clusterworkflow, profilehandler (`machine_cidr`, `service_cidr`, `pod_cidr`, `host_prefix` profile fields already exist in `Profile`).

**E2E** — new `tests/e2e/seaplane/cluster_networking_test.go` (profile-gated full-lifecycle style), new profile `seaplane-custom-cidrs`:
1. Positive: create cluster with custom, non-overlapping `pod_cidr`/`service_cidr` and `host_prefix` (e.g. pod `10.200.0.0/16`, service `172.30.0.0/16`, host prefix 24). Constraint: with the byovpc flow, `machine_cidr` must contain the VPC CIDR (default `10.0.0.0/16` via `SEAPLANE_VPC_CIDR`) — either keep machine CIDR = VPC CIDR or set a matching custom VPC CIDR in the profile.
2. Verify via Platform API: `cluster.Spec.HostedCluster.Networking.{MachineNetwork,ClusterNetwork,ServiceNetwork}` reflect the values (including `HostPrefix` inside clusterNetwork entries); verify TF outputs.
3. Wait for cluster `Ready` (networking misconfiguration would fail provisioning — a strong end-to-end signal).
4. Negative day-1 (V1 `negative_day_one_test.go` parity): invalid CIDR format; machine/service overlap; machine/pod overlap; service/pod overlap; host prefix out of range — each rejected by provider validation (no cluster needed → also covered in subsystem).
5. API-side negative (V1 `[id:70970]` analog): machine CIDR that does not contain the VPC subnet → Platform API / provisioning error.
6. Immutability (V1 `cluster_edit_test.go` parity): changing any CIDR or host prefix after create → plan shows replacement (subsystem plan assertion).

**Subsystem** — `subsystem/seaplane/cluster_resource_test.go`: context "Cluster networking" — encoding assertions (POST body contains JSON arrays), validation diags for each negative rule, replacement-on-change plan.

**Estimated scope:** medium-large.

---

## 3. CI / test wiring summary

| Piece | Changes |
|-------|---------|
| `tests/ci/profiles/tf_seaplane_cluster_profiles.yml` | New profiles: `seaplane-nodepool-tags` (or reuse `tagging: true`), `seaplane-disable-caps`, `seaplane-custom-cidrs` |
| `tests/e2e/seaplane/` | New files: `nodepool_tags_test.go`, `nodepool_gpu_test.go`, `cluster_capabilities_test.go`, `cluster_networking_test.go` |
| `tests/utils/exec/seaplane_cluster.go` | Add `EnabledCapabilities`, `DisabledCapabilities`, `MachineCIDR`, `ServiceCIDR`, `PodCIDR`, `HostPrefix` (+ outputs) |
| `tests/utils/exec/seaplane_nodepool.go` | Fix `Tags` to `*map[string]string`; add tags output |
| `tests/utils/clusterworkflow/seaplane.go` | Map new profile fields → args |
| `tests/utils/profilehandler/` | New profile fields + getters for capabilities; CIDR fields already exist |
| `subsystem/seaplane/` | New contexts in `cluster_resource_test.go` / `nodepool_resource_test.go` |

All new tests use the `ci.SeaplaneValidated` label and profile/env gating (skip unless the relevant profile is active), following `nodepool_autoscaling_test.go`.

---

## 4. Sequencing / PR breakdown

1. **PR 1 — ROSAENG-65173 node pool tags** (provider + e2e + subsystem). Mirrors 65293; unblocks the "is 73492 a duplicate" decision by giving TF-side parity with 73638.
2. **PR 2 — ROSAENG-65204 GPU machine pools** (e2e only + CI profile). Smallest; can land independently.
3. **PR 3 — ROSAENG-65243 image registry capabilities** (pathbind + regen + handler + e2e + subsystem). Requires upstream draft (already on rosa-seaplane-api main).
4. **PR 4 — ROSAENG-65120 custom network CIDRs** (consumer aliases + regen + handler encoding/validation + e2e + subsystem). Largest; benefits from patterns proven in PR 3.

PRs 1–2 can proceed in parallel; 3 and 4 share the pathbind/regen/handler workflow and are ordered to reduce rebase churn.

---

## 5. Decisions (resolved after review)

1. **Upstream draft availability (capabilities):** ~~local replace stale~~ **Resolved:** `../rosa-seaplane-api` updated to the branch that carries `capabilities` in the draft; run `go mod tidy` (+ vendor if applicable) so dependencies pick it up. Regenerate against the updated local repo.
2. **Immutable semantics:** **Confirmed:** capabilities and networking are set only at cluster create time and immutable after that. TF must plan replacement on change — generated plan modifiers (`immutable: true`) must produce `RequiresReplace`; assert in subsystem plan tests.
3. **host_prefix representation:** **Agreed:** the friendly `host_prefix` attribute must round-trip (flatten) correctly when the API returns defaults.
4. **Machine CIDR versus byovpc VPC:** **Agreed:** custom machine CIDR profiles keep `machine_cidr ⊇ VPC CIDR` or set `SEAPLANE_VPC_CIDR` accordingly.
5. **GPU cost/quota:** **Scoped down:** the GPU test just confirms the TF flow is not restricted from defining a GPU machine type (no deep EC2/GPU verification, no expensive dedicated provisioning beyond one small GPU nodepool if quota allows; otherwise dry-plan level validation).
6. **Nodepool `resource_tags` versus `tags` duality:** **Agreed:** keep the raw JSON passthrough attribute as a documented escape hatch; ensure state round-trip doesn't conflict between the two.
7. **V1/V2 behavioral divergence:** **Agreed:** nodepool tags are immutable in V1 but mutable in V2 — call this out in the e2e test and docs.
8. **Subsystem registry:** **Agreed:** all schema changes ship with subsystem coverage in the same PR (no allowlist entries for new types).

---

## 6. Verification per PR

- `make generate-seaplane` (where pathbind changes) + `go build ./...`
- `make pre-push-checks` (fmt, lint, unit, subsystem, gitleaks, subsystem-registry)
- Regenerate docs (`make docs`) and review `docs/resources/{cluster,nodepool}_seaplane.md`
- Local e2e smoke: `SEAPLANE_URL=… SEAPLANE_RELEASE_IMAGE=… make e2e-seaplane` with the new profile (or `-focus` on the new specs)
- PR description: reproducible validation steps per the template; link the epic; note the V1→V2 divergences (tags mutability)

---

## Appendix A — reference file map

**rosa CLI (../rosa):**
- Capabilities flags/validation: `cmd/create/cluster/seaplane.go` (`encodeSeaplaneClusterCapabilities`), `pkg/seaplane/pathbind/cluster_create_gen.go`
- Nodepool tags: `cmd/create/machinepool/seaplane.go` (`parseSeaplaneResourceTags`, `maxAWSResourceTags`)
- Networking bridge: `cmd/create/cluster/seaplane.go` ~L360–385
- V2 capabilities e2e: `tests/e2e/seaplane_capabilities_test.go` (ROSAENG-68175)
- V1 tag e2e: `tests/e2e/hcp_machine_pool_test.go` (`[id:73638]`), `tests/e2e/test_rosacli_node_pool.go` (`[id:73492]`)
- V1 CIDR e2e: `tests/e2e/test_rosacli_cluster.go` (`[id:38857]` L2474, `[id:70970]` L3266)

**Seaplane API (../rosa-seaplane-api):**
- `api/v1alpha1/cluster_networking_types.go` (ClusterNetworking), `api/v1alpha1/zz_generated.passthrough.go` (Capabilities), `api/v1alpha1/public/awsnodepoolplatform_types.go` (ResourceTags)
- `clientset/pathbind/pathbind-draft.yaml` (field draft; capabilities present on upstream main)

**TF provider (this repo):**
- Pathbind config: `provider/seaplane/pathbind-overrides.yaml`; generation: `make generate-seaplane`
- Handlers: `provider/seaplane/cluster_handler.go` (`validateClusterTags` as the validation model), `provider/seaplane/nodepool_handler.go`
- Manifests: `tests/tf-manifests/rhcs/clusters/seaplane/`, `tests/tf-manifests/rhcs/node-pools/seaplane/`
- Exec services: `tests/utils/exec/seaplane_cluster.go`, `tests/utils/exec/seaplane_nodepool.go`
- Workflow/profiles: `tests/utils/clusterworkflow/seaplane.go`, `tests/utils/profilehandler/`, `tests/ci/profiles/tf_seaplane_cluster_profiles.yml`
- E2E: `tests/e2e/seaplane/` (sanity, proxy, nodepool_autoscaling, cluster_day1/day2 as patterns)
- Subsystem: `subsystem/seaplane/{cluster,nodepool}_resource_test.go`
