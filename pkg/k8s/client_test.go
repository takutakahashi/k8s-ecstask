package k8s

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestNewClientFromConfig(t *testing.T) {
	// Create a fake config for testing
	config := &rest.Config{
		Host: "https://example.com",
	}

	client, err := NewClientFromConfig(config)
	if err != nil {
		t.Fatalf("NewClientFromConfig() failed: %v", err)
	}

	if client == nil {
		t.Fatal("NewClientFromConfig() returned nil client")
	}

	if client.Config() != config {
		t.Error("NewClientFromConfig() config mismatch")
	}
}

func TestClient_Config(t *testing.T) {
	config := &rest.Config{
		Host: "https://example.com",
	}

	client := &Client{
		Clientset: fake.NewSimpleClientset(),
		config:    config,
	}

	if client.Config() != config {
		t.Error("Client.Config() returned wrong config")
	}
}
