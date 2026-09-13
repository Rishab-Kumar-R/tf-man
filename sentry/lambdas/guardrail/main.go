package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/smithy-go"
)

const openCidr = "0.0.0.0/0"

type cidrItem struct {
	CidrIp string `json:"cidrIp"`
}

type ipPermissionItem struct {
	IpProtocol string `json:"ipProtocol"`
	FromPort   *int32 `json:"fromPort"`
	ToPort     *int32 `json:"toPort"`
	IpRanges   struct {
		Items []cidrItem `json:"items"`
	} `json:"ipRanges"`
}

type cloudTrailDetail struct {
	EventName         string `json:"eventName"`
	RequestParameters struct {
		GroupId       string `json:"groupId"`
		IpPermissions struct {
			Items []ipPermissionItem `json:"items"`
		} `json:"ipPermissions"`
	} `json:"requestParameters"`
	UserIdentity struct {
		Arn string `json:"arn"`
	} `json:"userIdentity"`
}

func handler(ctx context.Context, event events.CloudWatchEvent) error {
	var detail cloudTrailDetail
	if err := json.Unmarshal(event.Detail, &detail); err != nil {
		return fmt.Errorf("failed to parse event detail: %w", err)
	}

	groupID := detail.RequestParameters.GroupId
	if groupID == "" {
		log.Printf("no groupId in event, skipping")
		return nil
	}

	var offending []ec2types.IpPermission
	for _, item := range detail.RequestParameters.IpPermissions.Items {
		var openRanges []ec2types.IpRange
		for _, r := range item.IpRanges.Items {
			if r.CidrIp == openCidr {
				openRanges = append(openRanges, ec2types.IpRange{CidrIp: aws.String(openCidr)})
			}
		}

		if len(openRanges) == 0 {
			continue
		}

		offending = append(offending, ec2types.IpPermission{
			IpProtocol: aws.String(item.IpProtocol),
			FromPort:   item.FromPort,
			ToPort:     item.ToPort,
			IpRanges:   openRanges,
		})
	}

	if len(offending) == 0 {
		log.Printf("ingress rule on %s did not include 0.0.0.0/0, no action taken", groupID)
		return nil
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load AWS config: %w", err)
	}

	ec2Client := ec2.NewFromConfig(cfg)
	_, err = ec2Client.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
		GroupId:       aws.String(groupID),
		IpPermissions: offending,
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "InvalidPermission.NotFound" {
			log.Printf("rule on %s already removed (likely duplicate/backlogged event), no action needed", groupID)
			return nil
		}
		return fmt.Errorf("failed to revoke open ingress on %s: %w", groupID, err)
	}

	log.Printf("revoked 0.0.0.0/0 ingress rule(s) on %s (opened by %s)", groupID, detail.UserIdentity.Arn)

	snsClient := sns.NewFromConfig(cfg)
	topicArn := os.Getenv("SNS_TOPIC_ARN")
	if topicArn != "" {
		message := fmt.Sprintf(
			"Guardrail auto-reverted a 0.0.0.0/0 ingress rule on security group %s.\nOpened by: %s",
			groupID, detail.UserIdentity.Arn,
		)
		_, err = snsClient.Publish(ctx, &sns.PublishInput{
			TopicArn: aws.String(topicArn),
			Subject:  aws.String("Sentry Guardrail: open ingress reverted"),
			Message:  aws.String(message),
		})
		if err != nil {
			log.Printf("warning: failed to publish SNS notification: %v", err)
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
