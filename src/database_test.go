package main

import (
	"testing"
	"os"
	"path/filepath"
)

func TestDatabaseLogic(t *testing.T) {
	// Create a temporary database file for testing
	tmpFile := filepath.Join(os.TempDir(), "test-ovpn-radius.db")
	defer os.Remove(tmpFile)

	// Initialize test database
	repository, err := InitializeDatabase(true)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Test case 1: Create new client
	client1 := OVPNClient{
		Id:         "192.168.1.1:1234",
		CommonName: "testuser",
		ClassName:  "testclass",
	}

	_, err = repository.Create(client1)
	if err != nil {
		t.Fatalf("Failed to create new client: %v", err)
	}

	// Test case 2: Try to create duplicate (should fail with ErrDuplicate)
	_, err = repository.Create(client1)
	if err != ErrDuplicate {
		t.Fatalf("Expected ErrDuplicate, got: %v", err)
	}

	// Test case 3: Get existing record
	existing, err := repository.GetById(client1.Id)
	if err != nil {
		t.Fatalf("Failed to get existing client: %v", err)
	}

	if existing.Id != client1.Id || existing.CommonName != client1.CommonName {
		t.Fatalf("Retrieved client doesn't match: got %+v, want %+v", existing, client1)
	}

	// Test case 4: Update existing record (simulates TLS renegotiation handling)
	existing.ClassName = "updatedclass"
	_, err = repository.Update(*existing)
	if err != nil {
		t.Fatalf("Failed to update existing client: %v", err)
	}

	// Verify the update
	updated, err := repository.GetById(client1.Id)
	if err != nil {
		t.Fatalf("Failed to get updated client: %v", err)
	}

	if updated.ClassName != "updatedclass" {
		t.Fatalf("Update failed: got class %s, want %s", updated.ClassName, "updatedclass")
	}

	// Test case 5: Test the logic our fix implements
	// Simulate what happens during TLS renegotiation
	clientId := "192.168.1.2:5678"
	
	// First authentication - create new record
	newClient := OVPNClient{
		Id:         clientId,
		CommonName: "renegotiate_user",
		ClassName:  "initialclass",
	}
	
	_, err = repository.Create(newClient)
	if err != nil {
		t.Fatalf("Failed to create client for renegotiation test: %v", err)
	}
	
	// Simulate TLS renegotiation - same client ID, possibly different class
	existingForRenegotiation, errGet := repository.GetById(clientId)
	if errGet != nil && errGet != ErrNotExists {
		t.Fatalf("Failed to check existing client during renegotiation: %v", errGet)
	}
	
	if existingForRenegotiation != nil {
		// Update existing record (this is what our fix does)
		existingForRenegotiation.ClassName = "renegotiated_class"
		_, errUpdate := repository.Update(*existingForRenegotiation)
		if errUpdate != nil {
			t.Fatalf("Failed to update during renegotiation: %v", errUpdate)
		}
	} else {
		t.Fatalf("Expected existing client for renegotiation test")
	}
	
	// Verify the renegotiation update worked
	finalClient, err := repository.GetById(clientId)
	if err != nil {
		t.Fatalf("Failed to get client after renegotiation: %v", err)
	}
	
	if finalClient.ClassName != "renegotiated_class" {
		t.Fatalf("Renegotiation update failed: got class %s, want %s", finalClient.ClassName, "renegotiated_class")
	}

	t.Logf("All database logic tests passed successfully")
}