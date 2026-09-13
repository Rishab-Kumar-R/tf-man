package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

const flagTagKey = "sentry-flagged-at"

type reaper struct {
	ec2Client   *ec2.Client
	snsClient   *sns.Client
	topicArn    string
	gracePeriod time.Duration
}

func (r *reaper) notify(ctx context.Context, message string) {
	if r.topicArn == "" {
		return
	}

	_, err := r.snsClient.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(r.topicArn),
		Subject:  aws.String("Sentry Reaper"),
		Message:  aws.String(message),
	})
	if err != nil {
		log.Printf("warning: failed to publish SNS notification: %v", err)
	}
}

func findFlagTag(tags []ec2types.Tag) (string, bool) {
	for _, t := range tags {
		if aws.ToString(t.Key) == flagTagKey {
			return aws.ToString(t.Value), true
		}
	}

	return "", false
}

func (r *reaper) sweep(
	ctx context.Context,
	resourceID, resourceType string,
	tags []ec2types.Tag,
	deleteFn func(ctx context.Context) error,
) {
	now := time.Now().UTC()
	value, flagged := findFlagTag(tags)

	if !flagged {
		_, err := r.ec2Client.CreateTags(ctx, &ec2.CreateTagsInput{
			Resources: []string{resourceID},
			Tags: []ec2types.Tag{
				{Key: aws.String(flagTagKey), Value: aws.String(now.Format(time.RFC3339))},
			},
		})
		if err != nil {
			log.Printf("failed to flag orphaned %s %s: %v", resourceType, resourceID, err)
			return
		}

		log.Printf("flagged orphaned %s %s for review", resourceType, resourceID)
		return
	}

	flaggedAt, err := time.Parse(time.RFC3339, value)
	if err != nil {
		log.Printf("could not parse flag timestamp on %s %s: %v", resourceType, resourceID, err)
		return
	}

	elapsed := now.Sub(flaggedAt)
	if elapsed < r.gracePeriod {
		log.Printf("%s %s still within grace period (%s elapsed of %s)", resourceType, resourceID, elapsed, r.gracePeriod)
		return
	}

	if err := deleteFn(ctx); err != nil {
		log.Printf("failed to reap %s %s: %v", resourceType, resourceID, err)
		return
	}

	msg := fmt.Sprintf("Reaped orphaned %s %s after %s grace period", resourceType, resourceID, r.gracePeriod)
	log.Print(msg)
	r.notify(ctx, msg)
}

func (r *reaper) sweepVolumes(ctx context.Context) {
	out, err := r.ec2Client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("status"), Values: []string{"available"}},
		},
	})

	if err != nil {
		log.Printf("failed to describe volumes: %v", err)
		return
	}

	for _, v := range out.Volumes {
		volumeID := aws.ToString(v.VolumeId)
		r.sweep(ctx, volumeID, "ebs-volume", v.Tags, func(ctx context.Context) error {
			_, err := r.ec2Client.DeleteVolume(ctx, &ec2.DeleteVolumeInput{VolumeId: aws.String(volumeID)})
			return err
		})
	}
}

func (r *reaper) sweepAddresses(ctx context.Context) {
	out, err := r.ec2Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		log.Printf("failed to describe addresses: %v", err)
		return
	}

	for _, a := range out.Addresses {
		if aws.ToString(a.AssociationId) != "" {
			continue
		}

		allocationID := aws.ToString(a.AllocationId)
		if allocationID == "" {
			continue
		}

		r.sweep(ctx, allocationID, "eip", a.Tags, func(ctx context.Context) error {
			_, err := r.ec2Client.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{AllocationId: aws.String(allocationID)})
			return err
		})
	}
}

func handler(ctx context.Context, _ events.CloudWatchEvent) error {
	gracePeriodStr := os.Getenv("GRACE_PERIOD_SECONDS")
	gracePeriodSeconds, err := strconv.Atoi(gracePeriodStr)
	if err != nil {
		return fmt.Errorf("invalid GRACE_PERIOD_SECONDS env var %q: %w", gracePeriodStr, err)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load AWS config: %w", err)
	}

	r := &reaper{
		ec2Client:   ec2.NewFromConfig(cfg),
		snsClient:   sns.NewFromConfig(cfg),
		topicArn:    os.Getenv("SNS_TOPIC_ARN"),
		gracePeriod: time.Duration(gracePeriodSeconds) * time.Second,
	}

	r.sweepVolumes(ctx)
	r.sweepAddresses(ctx)

	return nil
}

func main() {
	lambda.Start(handler)
}
