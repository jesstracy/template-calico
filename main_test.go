package main

import (
	"errors"
	"testing"
)

func TestReplaceTemplateValue(t *testing.T) {
	testCases := []struct {
		name          string
		oldLine       string
		location		  []int
		expected      string
		expectedErr   error
	}{
		{
			name: "Release namespace",
			oldLine: "  namespace: {{ .Release.Namespace }}",
			location: []int{13, 37, 15, 35},
			expected: "  namespace: test-namespace",
			expectedErr: nil,
		},
		{
			name: "Release namespace with stuff after it",
			oldLine: "    name: {{ .Release.Name }}-network-policy",
			location: []int{10, 29, 12, 27},
			expected: "    name: test-release-name-network-policy",
			expectedErr: nil,
		},
		{
			name: "Not release or values",
			oldLine: "test: {{ notRight }}-network-policy",
			location: []int{6, 20, 8, 18},
			expected: "",
			expectedErr: errors.New("cannot template value notRight"),
		},
	}
	for _, tc := range testCases {
		tc := tc
		r := &runner{args: args{release: "test-release-name", namespace: "test-namespace"}}
		t.Run(tc.name, func(subT *testing.T) {
			result, err := r.replaceTemplateValue(tc.oldLine, tc.location)
			if err != nil {
				if err.Error() != tc.expectedErr.Error() {
					subT.Fatalf("expected error %s, got %s\n", tc.expectedErr.Error(), err.Error())
				}
			} else {
				if result != tc.expected {
					subT.Errorf("expected line %s, got %s\n", tc.expected, result)
				}
			}
		})
	}
}
