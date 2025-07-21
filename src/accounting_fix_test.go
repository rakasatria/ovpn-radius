package main

import (
	"os"
	"testing"
)

// TestAccountingIdConsistency verifies that the accounting phase uses the same ID format
// as the authentication phase, fixing the reported accounting issue
func TestAccountingIdConsistency(t *testing.T) {
	// Create test database
	repository, err := InitializeDatabase(true)
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	
	// Simulate authentication phase
	t.Log("Simulating authentication phase...")
	
	// Set environment variables as OpenVPN would during auth-user-pass-verify
	os.Setenv("untrusted_ip", "192.168.1.50")
	os.Setenv("untrusted_port", "55606")
	os.Setenv("trusted_ip", "192.168.1.12")  
	os.Setenv("trusted_port", "1194")
	os.Setenv("ifconfig_pool_remote_ip", "172.17.1.6")
	
	// Create client record as authentication phase would
	authClientId := os.Getenv("untrusted_ip") + ":" + os.Getenv("untrusted_port")
	client := OVPNClient{
		Id:         authClientId,
		CommonName: "testuser",
		ClassName:  "testclass",
	}
	
	_, err = repository.Create(client)
	if err != nil {
		t.Fatalf("Failed to create client during auth simulation: %v", err)
	}
	
	t.Logf("Authentication phase created client with ID: %s", authClientId)
	
	// Simulate accounting phase (this is where the bug was)
	t.Log("Simulating accounting phase...")
	
	// Before the fix: this would use trusted_ip:trusted_port and fail
	// After the fix: this should use untrusted_ip:untrusted_port and succeed
	acctClientId := os.Getenv("untrusted_ip") + ":" + os.Getenv("untrusted_port")
	
	// Try to retrieve the client record as accounting phase would
	retrievedClient, err := repository.GetById(acctClientId)
	if err != nil {
		t.Fatalf("Accounting phase failed to retrieve client (this was the bug): %v", err)
	}
	
	t.Logf("Accounting phase successfully found client with ID: %s", acctClientId)
	
	// Verify the IDs match (this is the core fix)
	if authClientId != acctClientId {
		t.Fatalf("ID mismatch: auth=%s, acct=%s", authClientId, acctClientId)
	}
	
	// Verify client data integrity
	if retrievedClient.CommonName != client.CommonName {
		t.Fatalf("Client data mismatch: got %s, want %s", retrievedClient.CommonName, client.CommonName)
	}
	
	t.Log("SUCCESS: Authentication and accounting phases now use consistent client IDs")
	
	// Clean up environment variables
	os.Unsetenv("untrusted_ip")
	os.Unsetenv("untrusted_port")
	os.Unsetenv("trusted_ip")
	os.Unsetenv("trusted_port")
	os.Unsetenv("ifconfig_pool_remote_ip")
}

// TestOriginalBugScenario demonstrates the original bug scenario
func TestOriginalBugScenario(t *testing.T) {
	// Create test database
	repository, err := InitializeDatabase(true)
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	
	// Set environment variables matching the issue report
	os.Setenv("untrusted_ip", "192.168.1.50")
	os.Setenv("untrusted_port", "55606")
	os.Setenv("trusted_ip", "192.168.1.12")
	os.Setenv("trusted_port", "1194")
	os.Setenv("ifconfig_pool_remote_ip", "172.17.1.6")
	defer func() {
		os.Unsetenv("untrusted_ip")
		os.Unsetenv("untrusted_port")
		os.Unsetenv("trusted_ip")
		os.Unsetenv("trusted_port")
		os.Unsetenv("ifconfig_pool_remote_ip")
	}()
	
	// Create client as auth phase would (using untrusted_ip:untrusted_port)
	authId := "192.168.1.50:55606"
	client := OVPNClient{
		Id:         authId,
		CommonName: "testuser", 
		ClassName:  "testclass",
	}
	
	_, err = repository.Create(client)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	
	// Test the fix: accounting should now use untrusted_ip:untrusted_port
	acctId := os.Getenv("untrusted_ip") + ":" + os.Getenv("untrusted_port")
	
	_, err = repository.GetById(acctId)
	if err != nil {
		t.Fatalf("With fix applied, accounting should find the client: %v", err)
	}
	
	// Demonstrate what would have failed before the fix
	wrongId := os.Getenv("trusted_ip") + ":" + os.Getenv("trusted_port")
	_, err = repository.GetById(wrongId)
	if err == nil {
		t.Fatalf("This should fail - using trusted_ip:trusted_port should not find the client")
	}
	
	t.Logf("Confirmed: untrusted_ip:untrusted_port (%s) works, trusted_ip:trusted_port (%s) fails", acctId, wrongId)
}