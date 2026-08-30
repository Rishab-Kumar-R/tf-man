package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

var (
	ddbClient *dynamodb.Client
	tableName string
)

type Task struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}

	ddbClient = dynamodb.NewFromConfig(cfg)
	tableName = os.Getenv("TABLE_NAME")
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	method := req.RequestContext.HTTP.Method
	id := req.PathParameters["id"]

	switch {
	case method == "POST":
		return createTask(ctx, req.Body)
	case method == "GET" && id != "":
		return getTask(ctx, id)
	case method == "GET":
		return listTasks(ctx)
	case method == "DELETE" && id != "":
		return deleteTask(ctx, id)
	default:
		return respond(405, map[string]string{"error": "method not allowed"})
	}
}

func createTask(ctx context.Context, body string) (events.APIGatewayV2HTTPResponse, error) {
	var t Task
	if err := json.Unmarshal([]byte(body), &t); err != nil {
		return respond(400, map[string]string{"error": "invalid body"})
	}

	t.ID = uuid.New().String()
	if t.Status == "" {
		t.Status = "pending"
	}

	_, err := ddbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"id":     &types.AttributeValueMemberS{Value: t.ID},
			"title":  &types.AttributeValueMemberS{Value: t.Title},
			"status": &types.AttributeValueMemberS{Value: t.Status},
		},
	})

	if err != nil {
		return respond(500, map[string]string{"error": err.Error()})
	}

	return respond(201, t)
}

func getTask(ctx context.Context, id string) (events.APIGatewayV2HTTPResponse, error) {
	out, err := ddbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{
				Value: id,
			},
		},
	})

	if err != nil {
		return respond(500, map[string]string{"error": err.Error()})
	}
	if out.Item == nil {
		return respond(404, map[string]string{"error": "not found"})
	}

	task := Task{
		ID:     out.Item["id"].(*types.AttributeValueMemberS).Value,
		Title:  out.Item["title"].(*types.AttributeValueMemberS).Value,
		Status: out.Item["status"].(*types.AttributeValueMemberS).Value,
	}

	return respond(200, task)
}

func listTasks(ctx context.Context) (events.APIGatewayV2HTTPResponse, error) {
	out, err := ddbClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return respond(500, map[string]string{"error": err.Error()})
	}

	tasks := []Task{}
	for _, item := range out.Items {
		tasks = append(tasks, Task{
			ID:     item["id"].(*types.AttributeValueMemberS).Value,
			Title:  item["title"].(*types.AttributeValueMemberS).Value,
			Status: item["status"].(*types.AttributeValueMemberS).Value,
		})
	}

	return respond(200, tasks)
}

func deleteTask(ctx context.Context, id string) (events.APIGatewayV2HTTPResponse, error) {
	_, err := ddbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{
				Value: id,
			},
		},
	})

	if err != nil {
		return respond(500, map[string]string{"error": err.Error()})
	}

	return respond(204, nil)
}

func respond(status int, body any) (events.APIGatewayV2HTTPResponse, error) {
	var bodyStr string
	if body != nil {
		b, _ := json.Marshal(body)
		bodyStr = string(b)
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       bodyStr,
	}, nil
}

func main() {
	lambda.Start(handler)
}
