/*
Copyright (c) 2024 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package seaplane

import (
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"
	. "github.com/openshift-online/ocm-sdk-go/testing"

	. "github.com/terraform-redhat/terraform-provider-rhcs/subsystem/framework"
)

var _ = Describe("rhcs_nodepool_seaplane", func() {
	Context("Create operation", func() {
		var seaplaneServer *Server

		BeforeEach(func() {
			seaplaneServer = NewServer()
		})

		AfterEach(func() {
			seaplaneServer.Close()
		})

		It("creates a nodepool successfully", func() {
			clusterResponse := `{
				"metadata": {
					"uid": "test-cluster-id",
					"name": "test-cluster",
					"creationTimestamp": "2024-01-01T00:00:00Z"
				},
				"spec": {
					"hostedCluster": {
						"platform": {
							"type": "AWS",
							"aws": {
								"region": "us-east-1",
								"rolesRef": {
									"nodePoolManagementARN": "arn:aws:iam::123456789012:role/test-cluster-NodePool"
								}
							}
						}
					}
				},
				"status": {
					"phase": "Ready",
					"controlPlaneEndpoint": {
						"host": "api.test-cluster.example.com",
						"port": 6443
					}
				}
			}`

			nodepoolResponse := `{
				"metadata": {
					"uid": "test-nodepool-id",
					"name": "test-cluster.worker",
					"creationTimestamp": "2024-01-01T00:00:00Z"
				},
				"spec": {
					"autoRepair": true,
					"ec2InstanceMetadataHttpTokens": "required",
					"labels": {
						"workload-type": "general"
					},
					"nodePool": {
						"clusterName": "test-cluster",
						"release": {
							"image": ""
						},
						"platform": {
							"type": "AWS",
							"aws": {
								"subnet": {
									"id": "subnet-0abc123"
								},
								"instanceType": "m5.xlarge",
								"rootVolume": {
									"size": 100
								}
							}
						},
						"replicas": 3
					}
				},
				"status": {
					"phase": "Provisioning"
				}
			}`

			header := http.Header{"Content-Type": []string{"application/json"}}

			seaplaneServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/clusters/test-cluster"),
					RespondWith(http.StatusOK, clusterResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodPost, "/api/v0/nodepools"),
					RespondWith(http.StatusCreated, nodepoolResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/nodepools/test-cluster.worker"),
					RespondWith(http.StatusOK, nodepoolResponse, header),
				),
			)

			Terraform.Source(fmt.Sprintf(`
				provider "rhcs" {
					alias          = "sp"
							seaplane_url = "%s"
					aws_account_id = "123456789012"
					aws_region     = "us-east-1"
				}

				resource "rhcs_nodepool_seaplane" "test" {
					provider         = rhcs.sp
					cluster_id       = "test-cluster-id"
					cluster_name     = "test-cluster"
					name             = "test-cluster.worker"
					replicas         = 3
					subnet_id        = "subnet-0abc123"
					auto_repair      = true
					instance_type    = "m5.xlarge"
					size = 100
					labels = {
						"workload-type" = "general"
					}
				}
			`, seaplaneServer.URL()))

			runOutput := Terraform.Apply()
			Expect(runOutput.ExitCode).To(BeZero())
		})
	})

	Context("Update operation", func() {
		var seaplaneServer *Server

		BeforeEach(func() {
			seaplaneServer = NewServer()
		})

		AfterEach(func() {
			seaplaneServer.Close()
		})

		It("updates nodepool labels", func() {
			clusterResponse := `{
				"metadata": {
					"uid": "test-cluster-id",
					"name": "test-cluster",
					"creationTimestamp": "2024-01-01T00:00:00Z"
				},
				"spec": {
					"hostedCluster": {
						"platform": {
							"type": "AWS",
							"aws": {
								"region": "us-east-1",
								"rolesRef": {
									"nodePoolManagementARN": "arn:aws:iam::123456789012:role/test-cluster-NodePool"
								}
							}
						}
					}
				},
				"status": {
					"phase": "Ready",
					"controlPlaneEndpoint": {
						"host": "api.test-cluster.example.com",
						"port": 6443
					}
				}
			}`

			createResponse := `{
				"metadata": {
					"uid": "test-nodepool-id",
					"name": "test-cluster.worker",
					"creationTimestamp": "2024-01-01T00:00:00Z"
				},
				"spec": {
					"autoRepair": true,
					"labels": {
						"workload-type": "general"
					},
					"nodePool": {
						"clusterName": "test-cluster",
						"release": {
							"image": ""
						},
						"platform": {
							"type": "AWS",
							"aws": {
								"subnet": {
									"id": "subnet-0abc123"
								},
								"instanceType": "m5.xlarge",
								"rootVolume": {
									"size": 100
								}
							}
						},
						"replicas": 3
					}
				},
				"status": {
					"phase": "Ready"
				}
			}`

			updateResponse := `{
				"metadata": {
					"uid": "test-nodepool-id",
					"name": "test-cluster.worker",
					"creationTimestamp": "2024-01-01T00:00:00Z"
				},
				"spec": {
					"autoRepair": true,
					"labels": {
						"workload-type": "general",
						"environment": "production"
					},
					"nodePool": {
						"clusterName": "test-cluster",
						"release": {
							"image": ""
						},
						"platform": {
							"type": "AWS",
							"aws": {
								"subnet": {
									"id": "subnet-0abc123"
								},
								"instanceType": "m5.xlarge",
								"rootVolume": {
									"size": 100
								}
							}
						},
						"replicas": 3
					}
				},
				"status": {
					"phase": "Ready"
				}
			}`

			header := http.Header{"Content-Type": []string{"application/json"}}

			seaplaneServer.AppendHandlers(
				// First apply: create without extra labels
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/clusters/test-cluster"),
					RespondWith(http.StatusOK, clusterResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodPost, "/api/v0/nodepools"),
					RespondWith(http.StatusCreated, createResponse, header),
				),
				// Second apply: refresh (Read), then update to add labels.
				// PostExpand fetches the parent cluster again before Update.
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/nodepools/test-cluster.worker"),
					RespondWith(http.StatusOK, createResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/nodepools/test-cluster.worker"),
					RespondWith(http.StatusOK, createResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/clusters/test-cluster"),
					RespondWith(http.StatusOK, clusterResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodPut, "/api/v0/nodepools/test-cluster.worker"),
					RespondWith(http.StatusOK, updateResponse, header),
				),
			)

			// First apply: create with single label
			Terraform.Source(fmt.Sprintf(`
				provider "rhcs" {
					alias          = "sp"
					seaplane_url = "%s"
					aws_account_id = "123456789012"
					aws_region     = "us-east-1"
				}

				resource "rhcs_nodepool_seaplane" "test" {
					provider         = rhcs.sp
					cluster_id       = "test-cluster-id"
					cluster_name     = "test-cluster"
					name             = "test-cluster.worker"
					replicas         = 3
					subnet_id        = "subnet-0abc123"
					auto_repair      = true
					instance_type    = "m5.xlarge"
					size = 100
					labels = {
						"workload-type" = "general"
					}
				}
			`, seaplaneServer.URL()))

			runOutput := Terraform.Apply()
			Expect(runOutput.ExitCode).To(BeZero())

			// Second apply: update to add environment label
			Terraform.Source(fmt.Sprintf(`
				provider "rhcs" {
					alias          = "sp"
					seaplane_url = "%s"
					aws_account_id = "123456789012"
					aws_region     = "us-east-1"
				}

				resource "rhcs_nodepool_seaplane" "test" {
					provider         = rhcs.sp
					cluster_id       = "test-cluster-id"
					cluster_name     = "test-cluster"
					name             = "test-cluster.worker"
					replicas         = 3
					subnet_id        = "subnet-0abc123"
					auto_repair      = true
					instance_type    = "m5.xlarge"
					size = 100
					labels = {
						"workload-type" = "general",
						"environment"   = "production"
					}
				}
			`, seaplaneServer.URL()))

			runOutput = Terraform.Apply()
			Expect(runOutput.ExitCode).To(BeZero())
		})

		It("updates nodepool auto_repair", func() {
			clusterResponse := `{
				"metadata": {
					"uid": "test-cluster-id",
					"name": "test-cluster",
					"creationTimestamp": "2024-01-01T00:00:00Z"
				},
				"spec": {
					"hostedCluster": {
						"platform": {
							"type": "AWS",
							"aws": {
								"region": "us-east-1",
								"rolesRef": {
									"nodePoolManagementARN": "arn:aws:iam::123456789012:role/test-cluster-NodePool"
								}
							}
						}
					}
				},
				"status": {
					"phase": "Ready",
					"controlPlaneEndpoint": {
						"host": "api.test-cluster.example.com",
						"port": 6443
					}
				}
			}`

			createResponse := `{
				"metadata": {
					"uid": "test-nodepool-id",
					"name": "test-cluster.worker",
					"creationTimestamp": "2024-01-01T00:00:00Z"
				},
				"spec": {
					"autoRepair": true,
					"nodePool": {
						"clusterName": "test-cluster",
						"release": {
							"image": ""
						},
						"platform": {
							"type": "AWS",
							"aws": {
								"subnet": {
									"id": "subnet-0abc123"
								},
								"instanceType": "m5.xlarge",
								"rootVolume": {
									"size": 100
								}
							}
						},
						"replicas": 3
					}
				},
				"status": {
					"phase": "Ready"
				}
			}`

			updateResponse := `{
				"metadata": {
					"uid": "test-nodepool-id",
					"name": "test-cluster.worker",
					"creationTimestamp": "2024-01-01T00:00:00Z"
				},
				"spec": {
					"autoRepair": false,
					"nodePool": {
						"clusterName": "test-cluster",
						"release": {
							"image": ""
						},
						"platform": {
							"type": "AWS",
							"aws": {
								"subnet": {
									"id": "subnet-0abc123"
								},
								"instanceType": "m5.xlarge",
								"rootVolume": {
									"size": 100
								}
							}
						},
						"replicas": 3
					}
				},
				"status": {
					"phase": "Ready"
				}
			}`

			header := http.Header{"Content-Type": []string{"application/json"}}

			seaplaneServer.AppendHandlers(
				// First apply: create with auto_repair=true
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/clusters/test-cluster"),
					RespondWith(http.StatusOK, clusterResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodPost, "/api/v0/nodepools"),
					RespondWith(http.StatusCreated, createResponse, header),
				),
				// Second apply: refresh (Read), then update to disable auto_repair.
				// PostExpand fetches the parent cluster again before Update.
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/nodepools/test-cluster.worker"),
					RespondWith(http.StatusOK, createResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/nodepools/test-cluster.worker"),
					RespondWith(http.StatusOK, createResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/clusters/test-cluster"),
					RespondWith(http.StatusOK, clusterResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodPut, "/api/v0/nodepools/test-cluster.worker"),
					RespondWith(http.StatusOK, updateResponse, header),
				),
			)

			// First apply: create with auto_repair=true (default)
			Terraform.Source(fmt.Sprintf(`
				provider "rhcs" {
					alias          = "sp"
					seaplane_url = "%s"
					aws_account_id = "123456789012"
					aws_region     = "us-east-1"
				}

				resource "rhcs_nodepool_seaplane" "test" {
					provider         = rhcs.sp
					cluster_id       = "test-cluster-id"
					cluster_name     = "test-cluster"
					name             = "test-cluster.worker"
					replicas         = 3
					subnet_id        = "subnet-0abc123"
					auto_repair      = true
					instance_type    = "m5.xlarge"
					size = 100
				}
			`, seaplaneServer.URL()))

			runOutput := Terraform.Apply()
			Expect(runOutput.ExitCode).To(BeZero())

			// Second apply: update to disable auto_repair
			Terraform.Source(fmt.Sprintf(`
				provider "rhcs" {
					alias          = "sp"
					seaplane_url = "%s"
					aws_account_id = "123456789012"
					aws_region     = "us-east-1"
				}

				resource "rhcs_nodepool_seaplane" "test" {
					provider         = rhcs.sp
					cluster_id       = "test-cluster-id"
					cluster_name     = "test-cluster"
					name             = "test-cluster.worker"
					replicas         = 3
					subnet_id        = "subnet-0abc123"
					auto_repair      = false
					instance_type    = "m5.xlarge"
					size = 100
				}
			`, seaplaneServer.URL()))

			runOutput = Terraform.Apply()
			Expect(runOutput.ExitCode).To(BeZero())
		})
	})

	Context("NodePool tags", func() {
		var seaplaneServer *Server

		// clusterResponse is the parent cluster the PostExpand handler fetches
		// to compute the worker instance profile.
		clusterResponse := `{"metadata":{"uid":"test-cluster-id","name":"test-cluster"},"spec":{"hostedCluster":{"platform":{"type":"AWS","aws":{"region":"us-east-1","rolesRef":{"nodePoolManagementARN":"arn:aws:iam::123456789012:role/test-cluster-NodePool"}}}}}}`

		nodepoolWithTags := func(tags string) string {
			return fmt.Sprintf(`{
				"metadata": {
					"uid": "test-nodepool-id",
					"name": "test-cluster.worker",
					"creationTimestamp": "2024-01-01T00:00:00Z"
				},
				"spec": {
					"autoRepair": true,
					"nodePool": {
						"clusterName": "test-cluster",
						"platform": {
							"type": "AWS",
							"aws": {
									"subnet": { "id": "subnet-0abc123" },
									"instanceType": "m5.xlarge",
									"resourceTags": [%s]
								}
							},
							"replicas": 3
						}
					},
				"status": { "phase": "Ready" }
			}`, tags)
		}

		initialTagsJSON := `{"key":"aaa","value":"bbb"},{"key":"ccc","value":"ddd"}`
		updatedTagsJSON := `{"key":"aaa","value":"bbb"},{"key":"environment","value":"production"}`

		header := http.Header{"Content-Type": []string{"application/json"}}

		nodepoolSource := func(url, tags string) string {
			return fmt.Sprintf(`
				provider "rhcs" {
					alias          = "hf"
					seaplane_url = "%s"
					aws_account_id = "123456789012"
					aws_region     = "us-east-1"
				}

				resource "rhcs_nodepool_seaplane" "test" {
					provider         = rhcs.hf
					cluster_id       = "test-cluster-id"
					cluster_name     = "test-cluster"
					name             = "test-cluster.worker"
					replicas         = 3
					subnet_id        = "subnet-0abc123"
					auto_repair      = true
					instance_type    = "m5.xlarge"
					%s
				}
			`, url, tags)
		}

		BeforeEach(func() {
			seaplaneServer = NewServer()
		})

		AfterEach(func() {
			seaplaneServer.Close()
		})

		It("sends the tags as resourceTags on create", func() {
			// Route every nodepool read (post-create refresh, later plans) to the
			// tagged response so queued handlers are only consumed by writes.
			seaplaneServer.RouteToHandler(
				http.MethodGet, "/api/v0/nodepools/test-cluster.worker",
				RespondWith(http.StatusOK, nodepoolWithTags(initialTagsJSON), header),
			)
			seaplaneServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/clusters/test-cluster"),
					RespondWith(http.StatusOK, clusterResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodPost, "/api/v0/nodepools"),
					VerifyJQ(`.spec.nodePool.platform.aws.resourceTags | length`, 2),
					VerifyJQ(`.spec.nodePool.platform.aws.resourceTags[0].key`, "aaa"),
					VerifyJQ(`.spec.nodePool.platform.aws.resourceTags[0].value`, "bbb"),
					VerifyJQ(`.spec.nodePool.platform.aws.resourceTags[1].key`, "ccc"),
					VerifyJQ(`.spec.nodePool.platform.aws.resourceTags[1].value`, "ddd"),
					RespondWith(http.StatusCreated, nodepoolWithTags(initialTagsJSON), header),
				),
			)

			Terraform.Source(nodepoolSource(seaplaneServer.URL(), `
					tags = {
						"aaa" = "bbb"
						"ccc" = "ddd"
					}`))

			runOutput := Terraform.Apply()
			Expect(runOutput.ExitCode).To(BeZero())

			By("Planning again to confirm the tags round-trip without drift")
			planOutput := Terraform.Run("plan", "-no-color")
			Expect(planOutput.ExitCode).To(BeZero())
			planOutput.VerifyOutputContainsSubstring("No changes. Your infrastructure matches the configuration")
		})

		It("updates the tags in place when they change", func() {
			// V2 divergence from V1: node pool resource tags are mutable, so a tag
			// change must be applied as an update, not a replacement.
			seaplaneServer.RouteToHandler(
				http.MethodGet, "/api/v0/nodepools/test-cluster.worker",
				RespondWith(http.StatusOK, nodepoolWithTags(initialTagsJSON), header),
			)
			seaplaneServer.AppendHandlers(
				// First apply: create with tags
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/clusters/test-cluster"),
					RespondWith(http.StatusOK, clusterResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodPost, "/api/v0/nodepools"),
					RespondWith(http.StatusCreated, nodepoolWithTags(initialTagsJSON), header),
				),
				// Second apply: PostExpand fetches the parent cluster again before
				// Update.
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/clusters/test-cluster"),
					RespondWith(http.StatusOK, clusterResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodPut, "/api/v0/nodepools/test-cluster.worker"),
					VerifyJQ(`.spec.nodePool.platform.aws.resourceTags | length`, 2),
					VerifyJQ(`.spec.nodePool.platform.aws.resourceTags[0].key`, "aaa"),
					VerifyJQ(`.spec.nodePool.platform.aws.resourceTags[1].key`, "environment"),
					VerifyJQ(`.spec.nodePool.platform.aws.resourceTags[1].value`, "production"),
					RespondWith(http.StatusOK, nodepoolWithTags(updatedTagsJSON), header),
				),
			)

			Terraform.Source(nodepoolSource(seaplaneServer.URL(), `
					tags = {
						"aaa" = "bbb"
						"ccc" = "ddd"
					}`))

			runOutput := Terraform.Apply()
			Expect(runOutput.ExitCode).To(BeZero())

			Terraform.Source(nodepoolSource(seaplaneServer.URL(), `
					tags = {
						"aaa" = "bbb"
						"environment" = "production"
					}`))

			runOutput = Terraform.Apply()
			Expect(runOutput.ExitCode).To(BeZero())
			// In-place update, not a replacement.
			runOutput.VerifyOutputContainsSubstring("Apply complete! Resources: 0 added, 1 changed, 0 destroyed.")
		})

		It("rejects more tags than the Platform API accepts", func() {
			tags := make([]string, 0, 26)
			for i := range 26 {
				tags = append(tags, fmt.Sprintf("    \"tag%d\" = \"value\"", i))
			}

			Terraform.Source(nodepoolSource(seaplaneServer.URL(), fmt.Sprintf(`
					tags = {
%s
					}`, strings.Join(tags, "\n"))))

			runOutput := Terraform.Apply()
			Expect(runOutput.ExitCode).ToNot(BeZero())
			runOutput.VerifyErrorContainsSubstring("a maximum of 25 tags is supported, got 26")
		})

		It("rejects a tag key reserved for AWS", func() {
			Terraform.Source(nodepoolSource(seaplaneServer.URL(), `
					tags = {
						"aws:cloudformation:stack-name" = "mine"
					}`))

			runOutput := Terraform.Apply()
			Expect(runOutput.ExitCode).ToNot(BeZero())
			runOutput.VerifyErrorContainsSubstring("reserved for AWS use")
		})

		It("rejects using tags and resource_tags together", func() {
			Terraform.Source(nodepoolSource(seaplaneServer.URL(), `
					resource_tags = jsonencode([{ key = "aaa", value = "bbb" }])
					tags = {
						"ccc" = "ddd"
					}`))

			runOutput := Terraform.Apply()
			Expect(runOutput.ExitCode).ToNot(BeZero())
			runOutput.VerifyErrorContainsSubstring("tags and resource_tags are mutually exclusive")
		})
	})

	Context("Delete operation", func() {
		var seaplaneServer *Server

		BeforeEach(func() {
			seaplaneServer = NewServer()
		})

		AfterEach(func() {
			seaplaneServer.Close()
		})

		It("deletes a nodepool successfully", func() {
			clusterResponse := `{
				"metadata": {
					"uid": "test-cluster-id",
					"name": "test-cluster",
					"creationTimestamp": "2024-01-01T00:00:00Z"
				},
				"spec": {
					"hostedCluster": {
						"platform": {
							"type": "AWS",
							"aws": {
								"region": "us-east-1",
								"rolesRef": {
									"nodePoolManagementARN": "arn:aws:iam::123456789012:role/test-cluster-NodePool"
								}
							}
						}
					}
				},
				"status": {
					"phase": "Ready",
					"controlPlaneEndpoint": {
						"host": "api.test-cluster.example.com",
						"port": 6443
					}
				}
			}`

			createResponse := `{
				"metadata": {
					"uid": "test-nodepool-id",
					"name": "test-cluster.worker",
					"creationTimestamp": "2024-01-01T00:00:00Z"
				},
				"spec": {
					"autoRepair": true,
					"nodePool": {
						"clusterName": "test-cluster",
						"release": {
							"image": ""
						},
						"platform": {
							"type": "AWS",
							"aws": {
								"subnet": {
									"id": "subnet-0abc123"
								},
								"instanceType": "m5.xlarge",
								"rootVolume": {
									"size": 100
								}
							}
						},
						"replicas": 3
					}
				},
				"status": {
					"phase": "Ready"
				}
			}`

			header := http.Header{"Content-Type": []string{"application/json"}}

			seaplaneServer.AppendHandlers(
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/clusters/test-cluster"),
					RespondWith(http.StatusOK, clusterResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodPost, "/api/v0/nodepools"),
					RespondWith(http.StatusCreated, createResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodGet, "/api/v0/nodepools/test-cluster.worker"),
					RespondWith(http.StatusOK, createResponse, header),
				),
				CombineHandlers(
					VerifyRequest(http.MethodDelete, "/api/v0/nodepools/test-cluster.worker"),
					RespondWith(http.StatusNoContent, ""),
				),
			)

			Terraform.Source(fmt.Sprintf(`
				provider "rhcs" {
					alias          = "sp"
					seaplane_url = "%s"
					aws_account_id = "123456789012"
					aws_region     = "us-east-1"
				}

				resource "rhcs_nodepool_seaplane" "test" {
					provider         = rhcs.sp
					cluster_id       = "test-cluster-id"
					cluster_name     = "test-cluster"
					name             = "test-cluster.worker"
					replicas         = 3
					subnet_id        = "subnet-0abc123"
					auto_repair      = true
					instance_type    = "m5.xlarge"
					size = 100
				}
			`, seaplaneServer.URL()))

			runOutput := Terraform.Apply()
			Expect(runOutput.ExitCode).To(BeZero())

			runOutput = Terraform.Destroy()
			Expect(runOutput.ExitCode).To(BeZero())
		})
	})
})
