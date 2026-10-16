// Copyright Red Hat
// SPDX-License-Identifier: Apache-2.0

package seaplane

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	v1alpha1 "github.com/openshift-online/rosa-seaplane-api/api/v1alpha1/public"
	"github.com/openshift-online/rosa-seaplane-api/clientset/pathbind"
	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
)

// ── NodePool state tests ──────────────────────────────────────────────────────

func TestNodePoolState_BasicFields(t *testing.T) {
	subnetID := "subnet-abc"
	autoRepair := true
	replicas := int32(3)
	np := &v1alpha1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster.my-pool",
			Namespace: "account-123456789012",
			UID:       "np-uid-456",
		},
		Spec: v1alpha1.NodePoolSpec{
			AutoRepair: &autoRepair,
			NodePool: v1alpha1.NodePoolSpecPassthrough{
				ClusterName: "my-cluster",
				Replicas:    &replicas,
				Platform: v1alpha1.NodePoolPlatform{
					Type: hypershiftv1beta1.AWSPlatform,
					AWS: &v1alpha1.AWSNodePoolPlatform{
						InstanceType:    "m5.xlarge",
						InstanceProfile: "my-prefix-ROSA-Worker-Role",
						Subnet:          hypershiftv1beta1.AWSResourceReference{ID: &subnetID},
					},
				},
			},
		},
		Status: v1alpha1.NodePoolStatus{
			Phase: v1alpha1.NodePoolPhaseReady,
		},
	}

	// Verify the nodepool structure matches what the handler expects
	if np.Spec.NodePool.ClusterName != "my-cluster" {
		t.Errorf("ClusterName = %q, want my-cluster", np.Spec.NodePool.ClusterName)
	}
	if np.Name != "my-cluster.my-pool" {
		t.Errorf("Name = %q, want my-cluster.my-pool", np.Name)
	}
	if np.Spec.NodePool.Platform.AWS.InstanceType != "m5.xlarge" {
		t.Errorf("InstanceType = %q, want m5.xlarge", np.Spec.NodePool.Platform.AWS.InstanceType)
	}
	if np.Spec.NodePool.Platform.AWS.InstanceProfile != "my-prefix-ROSA-Worker-Role" {
		t.Errorf("InstanceProfile = %q, want my-prefix-ROSA-Worker-Role", np.Spec.NodePool.Platform.AWS.InstanceProfile)
	}
	if *np.Spec.NodePool.Replicas != 3 {
		t.Errorf("Replicas = %d, want 3", *np.Spec.NodePool.Replicas)
	}
	if np.Status.Phase != v1alpha1.NodePoolPhaseReady {
		t.Errorf("Phase = %q, want %q", np.Status.Phase, v1alpha1.NodePoolPhaseReady)
	}
}

func TestNodePoolState_AWSPlatformConfig(t *testing.T) {
	subnetID := "subnet-123"
	volumeSize := int64(100)

	np := &v1alpha1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster.pool-aws",
			UID:  "np-789",
		},
		Spec: v1alpha1.NodePoolSpec{
			NodePool: v1alpha1.NodePoolSpecPassthrough{
				ClusterName: "test-cluster",
				Platform: v1alpha1.NodePoolPlatform{
					Type: hypershiftv1beta1.AWSPlatform,
					AWS: &v1alpha1.AWSNodePoolPlatform{
						InstanceType: "m5.2xlarge",
						RootVolume: &v1alpha1.Volume{
							Size: volumeSize,
						},
						Subnet: hypershiftv1beta1.AWSResourceReference{ID: &subnetID},
					},
				},
			},
		},
	}

	if np.Spec.NodePool.Platform.AWS.RootVolume.Size != 100 {
		t.Errorf("RootVolume.Size = %d, want 100", np.Spec.NodePool.Platform.AWS.RootVolume.Size)
	}
	if *np.Spec.NodePool.Platform.AWS.Subnet.ID != "subnet-123" {
		t.Errorf("Subnet.ID = %q, want subnet-123", *np.Spec.NodePool.Platform.AWS.Subnet.ID)
	}
}

func TestNodePoolState_EC2InstanceMetadataHttpTokensExpansion(t *testing.T) {
	native := &NodePoolStateNative{EC2InstanceMetadataHttpTokens: "optional"}
	obj := &v1alpha1.NodePool{}

	if err := pathbind.Expand(context.Background(), native, obj); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if got := obj.Spec.EC2InstanceMetadataHttpTokens; got != v1alpha1.EC2InstanceMetadataHttpTokensOptional {
		t.Errorf("EC2InstanceMetadataHttpTokens = %q, want %q", got, v1alpha1.EC2InstanceMetadataHttpTokensOptional)
	}
}

func TestNodePoolResourceSchema_EC2InstanceMetadataHttpTokens(t *testing.T) {
	var resp resource.SchemaResponse
	NewNodePool().Schema(context.Background(), resource.SchemaRequest{}, &resp)

	attribute, ok := resp.Schema.Attributes["ec2_instance_metadata_http_tokens"].(schema.StringAttribute)
	if !ok {
		t.Fatal("ec2_instance_metadata_http_tokens is not a string attribute")
	}
	if !attribute.Optional {
		t.Error("ec2_instance_metadata_http_tokens should be optional so the API can apply its required default")
	}
	if attribute.Computed {
		t.Error("ec2_instance_metadata_http_tokens should not be provider-computed")
	}
	if len(attribute.PlanModifiers) == 0 {
		t.Error("ec2_instance_metadata_http_tokens should require replacement when changed")
	}
}

// ── validateNodePoolTags ──────────────────────────────────────────────────────

// nodePoolTagsMap builds the types.Map the generated schema produces for the
// nodepool `tags` attribute.
func nodePoolTagsMapForTest(t *testing.T, values map[string]string) types.Map {
	t.Helper()
	elements := make(map[string]attr.Value, len(values))
	for key, value := range values {
		elements[key] = types.StringValue(value)
	}
	tags, diags := types.MapValue(types.StringType, elements)
	if diags.HasError() {
		t.Fatalf("building tags map: %v", diags)
	}
	return tags
}

func TestValidateNodePoolTags_Valid(t *testing.T) {
	cases := []struct {
		name string
		tags types.Map
	}{
		{"null", types.MapNull(types.StringType)},
		{"unknown", types.MapUnknown(types.StringType)},
		{"empty", nodePoolTagsMapForTest(t, nil)},
		{"typical", nodePoolTagsMapForTest(t, map[string]string{"aaa": "bbb", "ccc": "ddd"})},
		// Keys and values may contain the extra characters AWS allows.
		{"allowed characters", nodePoolTagsMapForTest(t, map[string]string{"kubernetes.io/role": "worker+1@eu=west"})},
		{"maximum", nodePoolTagsMapForTest(t, generatedTags(maxNodePoolTags))},
	}
	for _, tc := range cases {
		if diags := validateNodePoolTags(context.Background(), tc.tags); diags.HasError() {
			t.Errorf("%s: unexpected error: %v", tc.name, diags.Errors())
		}
	}
}

func TestValidateNodePoolTags_Invalid(t *testing.T) {
	cases := []struct {
		name   string
		tags   types.Map
		detail string
	}{
		{
			name:   "too many tags",
			tags:   nodePoolTagsMapForTest(t, generatedTags(maxNodePoolTags+1)),
			detail: "a maximum of 25 tags is supported, got 26",
		},
		{
			name:   "empty value",
			tags:   nodePoolTagsMapForTest(t, map[string]string{"owner": ""}),
			detail: "tag key or tag value can not be empty",
		},
		{
			name:   "empty key",
			tags:   nodePoolTagsMapForTest(t, map[string]string{"": "platform"}),
			detail: "tag key or tag value can not be empty",
		},
		{
			name:   "reserved aws prefix",
			tags:   nodePoolTagsMapForTest(t, map[string]string{"aws:cloudformation:stack-name": "mine"}),
			detail: "reserved for AWS use",
		},
		{
			name:   "invalid key character",
			tags:   nodePoolTagsMapForTest(t, map[string]string{"owner!": "platform"}),
			detail: "expected a valid user tag key 'owner!'",
		},
		{
			name:   "invalid value character",
			tags:   nodePoolTagsMapForTest(t, map[string]string{"owner": "platform!"}),
			detail: "expected a valid user tag value for key 'owner'",
		},
		{
			name:   "overlong key",
			tags:   nodePoolTagsMapForTest(t, map[string]string{strings.Repeat("k", 129): "value"}),
			detail: "expected a valid user tag key",
		},
		{
			name:   "overlong value",
			tags:   nodePoolTagsMapForTest(t, map[string]string{"owner": strings.Repeat("v", 257)}),
			detail: "expected a valid user tag value for key 'owner'",
		},
	}
	for _, tc := range cases {
		diags := validateNodePoolTags(context.Background(), tc.tags)
		if !diags.HasError() {
			t.Errorf("%s: expected an error, got none", tc.name)
			continue
		}
		if detail := diags.Errors()[0].Detail(); !strings.Contains(detail, tc.detail) {
			t.Errorf("%s: detail = %q, want it to contain %q", tc.name, detail, tc.detail)
		}
	}
}

// ── nodePoolResourceTagsFromMap / nodePoolTagsMap ────────────────────────────

func TestNodePoolResourceTagsFromMap(t *testing.T) {
	tags := nodePoolResourceTagsFromMap(map[string]string{"zzz": "1", "aaa": "2"})
	if len(tags) != 2 {
		t.Fatalf("len(tags) = %d, want 2", len(tags))
	}
	// Keys are sorted so repeated applies produce identical request bodies.
	if tags[0].Key != "aaa" || tags[0].Value != "2" {
		t.Errorf("tags[0] = {%s: %s}, want {aaa: 2}", tags[0].Key, tags[0].Value)
	}
	if tags[1].Key != "zzz" || tags[1].Value != "1" {
		t.Errorf("tags[1] = {%s: %s}, want {zzz: 1}", tags[1].Key, tags[1].Value)
	}

	empty := nodePoolResourceTagsFromMap(map[string]string{})
	if empty == nil || len(empty) != 0 {
		t.Errorf("empty map should encode to an empty non-nil slice, got %v", empty)
	}
}

func TestNodePoolTagsMap_Flatten(t *testing.T) {
	flattened := nodePoolTagsMap([]hypershiftv1beta1.AWSResourceTag{
		{Key: "aaa", Value: "bbb"},
		{Key: "ccc", Value: "ddd"},
	})
	if flattened.IsNull() {
		t.Fatal("flattened map is null, want populated map")
	}
	var values map[string]string
	if diags := flattened.ElementsAs(context.Background(), &values, false); diags.HasError() {
		t.Fatalf("reading flattened map: %v", diags)
	}
	if values["aaa"] != "bbb" || values["ccc"] != "ddd" || len(values) != 2 {
		t.Errorf("values = %v, want map[aaa:bbb ccc:ddd]", values)
	}

	if got := nodePoolTagsMap(nil); !got.IsNull() {
		t.Error("nil tag list should flatten to a null map")
	}
	if got := nodePoolTagsMap([]hypershiftv1beta1.AWSResourceTag{}); !got.IsNull() {
		t.Error("empty tag list should flatten to a null map")
	}
}
