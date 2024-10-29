package taskutils

import (
	"testing"
)

func TestValidateFiles(t *testing.T) {
	tu := &TaskUtils{}

	tests := []struct {
		name          string
		files         map[string][]byte
		expectErr     bool
		expectedError string
	}{
		{
			name: "valid input and output files with description",
			files: map[string][]byte{
				"src/input/1.in":   {},
				"src/output/1.out": {},
				"src/input/2.in":   {},
				"src/output/2.out": {},
				"description.pdf":  {},
			},
			expectErr: false,
		},
		{
			name: "mismatched input and output counts",
			files: map[string][]byte{
				"src/input/1.in":   {},
				"src/output/1.out": {},
				"src/input/2.in":   {},
				"description.pdf":  {},
			},
			expectErr:     true,
			expectedError: "the number of input files must match the number of output files",
		},
		{
			name: "missing description file",
			files: map[string][]byte{
				"src/input/1.in":   {},
				"src/output/1.out": {},
			},
			expectErr:     true,
			expectedError: "a description file (description.pdf) is required",
		},
		{
			name: "incorrect input file format",
			files: map[string][]byte{
				"src/input/one.in": {},
				"src/output/1.out": {},
				"description.pdf":  {},
			},
			expectErr:     true,
			expectedError: "input file one.in does not match the required format {number}.in",
		},
		{
			name: "input and output numbers are not sequential",
			files: map[string][]byte{
				"src/input/1.in":   {},
				"src/input/3.in":   {},
				"src/output/1.out": {},
				"src/output/3.out": {},
				"description.pdf":  {},
			},
			expectErr:     true,
			expectedError: "input and output files must have matching numbers from 1 to 2",
		},
		{
			name: "description file has incorrect extension",
			files: map[string][]byte{
				"src/input/1.in":   {},
				"src/output/1.out": {},
				"description.txt":  {},
			},
			expectErr:     true,
			expectedError: "description must have a .pdf extension",
		},
		{
			name: "unrecognized file path",
			files: map[string][]byte{
				"src/input/1.in":   {},
				"src/output/1.out": {},
				"randomfile.txt":   {},
				"description.pdf":  {},
			},
			expectErr:     true,
			expectedError: "unrecognized file path randomfile.txt",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := tu.ValidateFiles(test.files)
			if test.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if err.Error() != test.expectedError {
					t.Errorf("expected error %v but got %v", test.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
