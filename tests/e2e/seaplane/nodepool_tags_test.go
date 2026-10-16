// Copyright Red Hat
// SPDX-License-Identifier: Apache-2.0

package seaplane

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1alpha1 "github.com/openshift-online/rosa-seaplane-api/api/v1alpha1/public"
	spplatform "github.com/openshift-online/rosa-seaplane-api/clientset/platform"

	"github.com/terraform-redhat/terraform-provider-rhcs/tests/ci"
	"github.com/terraform-redhat/terraform-provider-rhcs/tests/utils/clusterworkflow"
	"github.com/terraform-redhat/terraform-provider-rhcs/tests/utils/config"
	"github.com/terraform-redhat/terraform-provider-rhcs/tests/utils/constants"
	"github.com/terraform-redhat/terraform-provider-rhcs/tests/utils/exec"
	"github.com/terraform-redhat/terraform-provider-rhcs/tests/utils/profilehandler"
)

// ROSAENG-65173: custom node pool tags (V1 analog: id:72510).
//
// V2 divergence from V1: node pool tags are mutable in the Seaplane
// Platform API (spec.nodePool.platform.aws.resourceTags is applied in place),
// while V1 HCP rejected tag edits ("Attribute aws_node_pool.tags, cannot be
// changed from"). This test asserts the in-place update on purpose.
var _ = Describe("Seaplane NodePool tags", ci.Day2, ci.SeaplaneValidated, func() {
	It("creates a nodepool with tags and updates them in place", func(ctx SpecContext) {
		profileHandler, err := profilehandler.NewProfileHandlerFromYamlFile()
		Expect(err).NotTo(HaveOccurred())
		Expect(profileHandler.Profile().GetClusterType()).To(Equal(constants.SEAPLANE))
		if !profileHandler.Profile().IsTagging() {
			Skip("Test requires a Seaplane tagging profile")
		}

		workspace := profileHandler.Profile().GetName() + "-nodepool-tags"
		backend, err := clusterworkflow.New(profileHandler, "", workspace)
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func(ctx SpecContext) {
			destroyer, destroyErr := clusterworkflow.NewDestroyer(profileHandler, "", workspace)
			Expect(destroyErr).NotTo(HaveOccurred())
			Expect(destroyer.Destroy(ctx)).To(Succeed())
		}, GracePeriod(teardownGracePeriod))

		clusterID, err := exec.CreateAndWaitReady(ctx, backend.Lifecycle)
		Expect(err).NotTo(HaveOccurred())

		seaplaneURL := os.Getenv("SEAPLANE_URL")
		region := config.GetRegion()
		if region == "" {
			region = awsRegionRE.FindString(seaplaneURL)
		}
		Expect(region).NotTo(BeEmpty())

		seaplaneClient := mustBuildSeaplaneClient(seaplaneURL, region)
		cluster, err := seaplaneClient.SeaplaneV1alpha1().Clusters().Get(ctx, clusterID, spplatform.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(cluster.Spec.HostedCluster.Platform.AWS).NotTo(BeNil())
		Expect(cluster.Spec.HostedCluster.Platform.AWS.CloudProviderConfig).NotTo(BeNil())
		Expect(cluster.Spec.HostedCluster.Platform.AWS.CloudProviderConfig.Subnet).NotTo(BeNil())
		subnetID := cluster.Spec.HostedCluster.Platform.AWS.CloudProviderConfig.Subnet.ID
		Expect(subnetID).NotTo(BeNil())

		clusterName := cluster.Name
		nodePoolName := clusterName + "-tags"
		nodePoolService, err := exec.NewSeaplaneNodePoolService(workspace + "-nodepool")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func(_ SpecContext) {
			_, destroyErr := nodePoolService.Destroy()
			Expect(destroyErr).NotTo(HaveOccurred())
		}, GracePeriod(teardownGracePeriod))

		instanceType := os.Getenv("SEAPLANE_NODEPOOL_INSTANCE_TYPE")
		if instanceType == "" {
			instanceType = "m5.xlarge"
		}
		replicas := 1

		nodePools := seaplaneClient.SeaplaneV1alpha1().NodePools()
		resourceTagsFor := func(np *v1alpha1.NodePool) map[string]string {
			if np == nil || np.Spec.NodePool.Platform.AWS == nil {
				return nil
			}
			tags := map[string]string{}
			for _, tag := range np.Spec.NodePool.Platform.AWS.ResourceTags {
				tags[tag.Key] = tag.Value
			}
			return tags
		}
		assertTags := func(want map[string]string) func(*v1alpha1.NodePool) bool {
			return func(np *v1alpha1.NodePool) bool {
				if np == nil {
					return false
				}
				got := resourceTagsFor(np)
				if len(got) != len(want) {
					return false
				}
				for key, value := range want {
					if got[key] != value {
						return false
					}
				}
				return np.Status.Phase == v1alpha1.NodePoolPhaseReady
			}
		}

		By("Creating a nodepool with user tags")
		initialTags := map[string]string{"aaa": "bbb", "ccc": "ddd"}
		_, err = nodePoolService.Apply(&exec.SeaplaneNodePoolArgs{
			SeaplaneURL:  &seaplaneURL,
			AWSRegion:    &region,
			ClusterID:    &clusterID,
			ClusterName:  &clusterName,
			Name:         &nodePoolName,
			SubnetID:     subnetID,
			InstanceType: &instanceType,
			Replicas:     &replicas,
			Tags:         &initialTags,
		})
		Expect(err).NotTo(HaveOccurred())
		nodePoolTFOut, err := nodePoolService.Output()
		Expect(err).NotTo(HaveOccurred())
		Expect(nodePoolTFOut.Tags).To(Equal(initialTags))
		Expect(nodePools.WaitUntil(ctx, nodePoolName, assertTags(initialTags), pollInterval, nodepoolReadyTimeout)).To(Succeed())

		By("Updating the tags in place (V2: mutable, unlike V1 HCP)")
		updatedTags := map[string]string{"aaa": "bbb", "environment": "production"}
		_, err = nodePoolService.Apply(&exec.SeaplaneNodePoolArgs{
			SeaplaneURL:  &seaplaneURL,
			AWSRegion:    &region,
			ClusterID:    &clusterID,
			ClusterName:  &clusterName,
			Name:         &nodePoolName,
			SubnetID:     subnetID,
			InstanceType: &instanceType,
			Replicas:     &replicas,
			Tags:         &updatedTags,
		})
		Expect(err).NotTo(HaveOccurred())
		nodePoolTFOut, err = nodePoolService.Output()
		Expect(err).NotTo(HaveOccurred())
		Expect(nodePoolTFOut.Tags).To(Equal(updatedTags))
		Expect(nodePools.WaitUntil(ctx, nodePoolName, assertTags(updatedTags), pollInterval, nodepoolReadyTimeout)).To(Succeed())
	})
})

// Validation-only spec: the provider rejects invalid tags in PreExpand
// before any Platform API call, so no cluster is needed. Follows the V1
// day1-negative taxonomy.
var _ = Describe("Seaplane NodePool tags validation", ci.Day1Negative, ci.SeaplaneValidated, func() {
	It("rejects invalid nodepool tags during validation", ci.Day1Negative, func(ctx SpecContext) {
		profileHandler, err := profilehandler.NewProfileHandlerFromYamlFile()
		Expect(err).NotTo(HaveOccurred())
		Expect(profileHandler.Profile().GetClusterType()).To(Equal(constants.SEAPLANE))

		// Validation runs in PreExpand before any Platform API call, so a
		// dummy cluster identity is enough to reach the tag checks.
		seaplaneURL := os.Getenv("SEAPLANE_URL")
		Expect(seaplaneURL).NotTo(BeEmpty())
		region := config.GetRegion()
		if region == "" {
			region = awsRegionRE.FindString(seaplaneURL)
		}
		Expect(region).NotTo(BeEmpty())

		clusterID := "dummy-cluster-id"
		clusterName := "dummy-cluster"
		nodePoolName := clusterName + "-tags-invalid"
		instanceType := "m5.xlarge"
		replicas := 1
		subnetID := "subnet-dummy"

		invalidTags := []map[string]string{
			{"aws:reserved": "value"},
			{"owner!": "value"},
			{"owner": "value!"},
			{"": "value"},
		}
		expectedErrors := []string{
			"reserved for AWS use",
			"expected a valid user tag key 'owner!'",
			"expected a valid user tag value for key 'owner'",
			"tag key or tag value can not be empty",
		}

		for i, tags := range invalidTags {
			By("Applying a nodepool with invalid tags: " + expectedErrors[i])
			nodePoolService, serviceErr := exec.NewSeaplaneNodePoolService(
				profileHandler.Profile().GetName() + "-nodepool-tags-invalid")
			Expect(serviceErr).NotTo(HaveOccurred())

			_, applyErr := nodePoolService.Apply(&exec.SeaplaneNodePoolArgs{
				SeaplaneURL:  &seaplaneURL,
				AWSRegion:    &region,
				ClusterID:    &clusterID,
				ClusterName:  &clusterName,
				Name:         &nodePoolName,
				SubnetID:     &subnetID,
				InstanceType: &instanceType,
				Replicas:     &replicas,
				Tags:         &tags,
			})
			Expect(applyErr).To(HaveOccurred())
			Expect(applyErr.Error()).To(ContainSubstring(expectedErrors[i]))

			_, destroyErr := nodePoolService.Destroy()
			Expect(destroyErr).NotTo(HaveOccurred())
		}
	})
})
