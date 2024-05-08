package main

import (
	"context"
	"fmt"
	"log"
	"os"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

func init() {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		return
	}

	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fetchSecret := func(key string) string {
		accessRequest := &secretmanagerpb.AccessSecretVersionRequest{
			Name: fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectID, key),
		}
		result, err := client.AccessSecretVersion(ctx, accessRequest)
		if err != nil {
			log.Fatal(err)
		}
		return string(result.GetPayload().GetData())
	}

	token = fetchSecret(os.Getenv("TOKEN_SECRET_NAME"))
	pubKeyHex = fetchSecret(os.Getenv("PUBKEY_SECRET_NAME"))
}
