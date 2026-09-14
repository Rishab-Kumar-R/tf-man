package secrets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type DBCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	DBName   string `json:"dbname"`
}

func FetchDBCredentials(ctx context.Context, client *secretsmanager.Client, secretArn string) (*DBCredentials, error) {
	out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: &secretArn})
	if err != nil {
		return nil, fmt.Errorf("get secret value: %w", err)
	}

	var creds DBCredentials
	if err := json.Unmarshal([]byte(*out.SecretString), &creds); err != nil {
		return nil, fmt.Errorf("unmarshal db credentials: %w", err)
	}

	return &creds, nil
}
