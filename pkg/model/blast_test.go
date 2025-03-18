package model

import (
	"testing"
)

func TestBLASTP(t *testing.T) {

	VALID_TEST_SEQ := ">test_seq\nLGVGCEDGVVECWDTRSNNRVGLLDTIPGLVGGASLEDP\n"

	// Define test cases
	tests := []struct {
		name        string
		inputfasta  string
		mockDB      string
		expected    string
		shouldError bool
	}{
		{
			name:        "ValidInput",
			inputfasta:  VALID_TEST_SEQ,
			mockDB:      "/data/db/blastdb/pythium_prot_v3",
			expected:    "test_seq", // Adjust according to the expected output
			shouldError: false,
		},
		{
			name:        "EmptyInput",
			inputfasta:  "",
			mockDB:      "/data/db/blastdb/pythium_prot_v3",
			expected:    "",
			shouldError: true,
		},
		{
			name:        "InvalidDB",
			inputfasta:  VALID_TEST_SEQ,
			mockDB:      "invalid_db",
			expected:    "",
			shouldError: true,
		},
	}

	// Loop through each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the BLASTP function with the test case inputs
			result, err := BLASTP(tt.mockDB, tt.inputfasta)

			// Check if an error was expected
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected an error but got none")
				}
				return
			}

			// Check if the result contains the expected substring
			if !contains(result, tt.expected) {
				t.Errorf("Expected result to contain %q, but it didn't. Got: %q", tt.expected, result)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(str, substr string) bool {
	return len(str) >= len(substr) && str[0:len(substr)] == substr
}
