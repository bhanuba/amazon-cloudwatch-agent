// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"github.com/aws/amazon-cloudwatch-agent/tool/clean"
)

// Clean eks clusters if they have been open longer than 7 day
func main() {
	err := cleanCluster()
	if err != nil {
		log.Fatalf("errors cleaning %v", err)
	}
}

func cleanCluster() error {
	log.Print("Begin to clean EKS Clusters")
	ctx := context.Background()
	defaultConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	eksClient := eks.NewFromConfig(defaultConfig)

	terminateClusters(ctx, eksClient)
	return nil
}

func terminateClusters(ctx context.Context, client *eks.Client) {
	listClusterInput := eks.ListClustersInput{}
	expirationDateCluster := time.Now().UTC().Add(clean.KeepDurationOneWeek)
	clusters, err := client.ListClusters(ctx, &listClusterInput)
	if err != nil {
		log.Fatalf("could not get cluster list")
	}
	for _, cluster := range clusters.Clusters {
		describeClusterInput := eks.DescribeClusterInput{Name: aws.String(cluster)}
		describeClusterOutput, err := client.DescribeCluster(ctx, &describeClusterInput)
		if err != nil {
			return
		}
		if expirationDateCluster.After(*describeClusterOutput.Cluster.CreatedAt) && strings.HasPrefix(*describeClusterOutput.Cluster.Name, "cwagent-eks-integ-") {
			log.Printf("Try to delete cluster %s launch-date %s", cluster, *describeClusterOutput.Cluster.CreatedAt)
			describeNodegroupInput := eks.ListNodegroupsInput{ClusterName: aws.String(cluster)}
			nodeGroupOutput, err := client.ListNodegroups(ctx, &describeNodegroupInput)
			if err != nil {
				log.Printf("could not query node groups cluster %s err %v", cluster, err)
			}
			// it takes about 5 minutes to delete node groups
			// it will fail to delete cluster if we need to delete node groups
			// this will delete the cluster on next run the next day
			// I do not want to wait for node groups to be deleted
			// as it will increase the runtime of this cleaner
			for _, nodegroup := range nodeGroupOutput.Nodegroups {
				deleteNodegroupInput := eks.DeleteNodegroupInput{
					ClusterName:   aws.String(cluster),
					NodegroupName: aws.String(nodegroup),
				}
				_, err := client.DeleteNodegroup(ctx, &deleteNodegroupInput)
				if err != nil {
					log.Printf("could delete node groups %s cluster %s err %v", nodegroup, cluster, err)
				}
			}
			deleteClusterInput := eks.DeleteClusterInput{Name: aws.String(cluster)}
			_, err = client.DeleteCluster(ctx, &deleteClusterInput)
			if err != nil {
				log.Printf("could not delete cluster %s err %v", cluster, err)
			}
		}
	}
}
