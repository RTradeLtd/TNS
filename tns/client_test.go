package tns

import (
	"context"
	"testing"
)

const (
	clientMaAddr = "/ip4/127.0.0.1/tcp/10050"
)

func TestClient(t *testing.T) {
	client, err := NewClient(
		context.Background(),
		ClientOptions{true, nil, clientMaAddr},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
}
