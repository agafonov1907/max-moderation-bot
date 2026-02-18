package handler

import (
	"bufio"
	"strings"
	"testing"
)

func TestParseWordsFile(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedWords   []string
		expectedSkipped int
		wantErr         bool
	}{
		{
			name: "Valid input",
			input: `apple
banana
cherry`,
			expectedWords:   []string{"apple", "banana", "cherry"},
			expectedSkipped: 0,
			wantErr:         false,
		},
		{
			name: "Input with spaces and empty lines",
			input: `apple
   
banana with space
cherry
`,
			expectedWords:   []string{"apple", "cherry"},
			expectedSkipped: 1, // "banana with space" is skipped.
			wantErr:         false,
		},
		{
			name: "Input with surrounding whitespace",
			input: `  apple  
	banana	`,
			expectedWords:   []string{"apple", "banana"},
			expectedSkipped: 0,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := bufio.NewScanner(strings.NewReader(tt.input))
			words, skipped, err := parseWordsFile(scanner)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseWordsFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(words) != len(tt.expectedWords) {
				t.Errorf("parseWordsFile() got words count = %d, want %d", len(words), len(tt.expectedWords))
			}
			for i, w := range words {
				if w != tt.expectedWords[i] {
					t.Errorf("parseWordsFile() word[%d] = %v, want %v", i, w, tt.expectedWords[i])
				}
			}
			if skipped != tt.expectedSkipped {
				t.Errorf("parseWordsFile() skipped = %v, want %v", skipped, tt.expectedSkipped)
			}
		})
	}
}
