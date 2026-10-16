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

// ROSAENG-65204: GPU machine pools.
//
// V2 does not restrict node pool instance types (the rosa CLI and Platform API
// accept any EC2 type, including accelerated ones, and instance-type names are
// not validated client-side — an invalid type fails at AWS provisioning time,
// which is out of scope here). This test confirms the Terraform flow is not
// restricted from defining a GPU machine type either: a node pool created with
// a GPU instance type reaches Ready and reports the type back.
//
// The test is opt-in because GPU instances are expensive and quota-limited:
// set SEAPLANE_NODEPOOL_GPU_INSTANCE_TYPE (default suggestion: g5.xlarge)
// to run it.
var _ = Describe("Seaplane GPU machine pools", ci.SeaplaneValidated, func() {
	It("creates a nodepool with a GPU instance type", func(ctx SpecContext) {
		gpuInstanceType := os.Getenv("SEAPLANE_NODEPOOL_GPU_INSTANCE_TYPE")
		if gpuInstanceType == "" {
			Skip("Test is opt-in: set SEAPLANE_NODEPOOL_GPU_INSTANCE_TYPE (e.g. g5.xlarge) to run it")
		}

		profileHandler, err := profilehandler.NewProfileHandlerFromYamlFile()
		Expect(err).NotTo(HaveOccurred())
		Expect(profileHandler.Profile().GetClusterType()).To(Equal(constants.SEAPLANE))

		workspace := profileHandler.Profile().GetName() + "-gpu-machine-pool"
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
		nodePoolName := clusterName + "-gpu"
		nodePoolService, err := exec.NewSeaplaneNodePoolService(workspace + "-nodepool")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func(_ SpecContext) {
			_, destroyErr := nodePoolService.Destroy()
			Expect(destroyErr).NotTo(HaveOccurred())
		}, GracePeriod(teardownGracePeriod))

		By("Creating a nodepool with the GPU instance type " + gpuInstanceType)
		replicas := 1
		_, err = nodePoolService.Apply(&exec.SeaplaneNodePoolArgs{
			SeaplaneURL:  &seaplaneURL,
			AWSRegion:    &region,
			ClusterID:    &clusterID,
			ClusterName:  &clusterName,
			Name:         &nodePoolName,
			SubnetID:     subnetID,
			InstanceType: &gpuInstanceType,
			Replicas:     &replicas,
		})
		Expect(err).NotTo(HaveOccurred())

		nodePoolTFOut, err := nodePoolService.Output()
		Expect(err).NotTo(HaveOccurred())
		Expect(nodePoolTFOut.NodePoolID).NotTo(BeEmpty())

		By("Waiting for the GPU nodepool to reach Ready with the instance type preserved")
		nodePools := seaplaneClient.SeaplaneV1alpha1().NodePools()
		Expect(nodePools.WaitUntil(
			ctx, nodePoolName,
			func(np *v1alpha1.NodePool) bool {
				if np == nil || np.Spec.NodePool.Platform.AWS == nil {
					return false
				}
				return np.Spec.NodePool.Platform.AWS.InstanceType == gpuInstanceType &&
					np.Status.Phase == v1alpha1.NodePoolPhaseReady
			},
			pollInterval, nodepoolReadyTimeout,
		)).To(Succeed())
	})
})
