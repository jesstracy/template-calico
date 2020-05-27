package main

import (
	"errors"
	"testing"
)

func TestReplaceTemplateValue(t *testing.T) {
	r := &runner{
		args: args{release: "test-release-name", namespace: "test-namespace"},
		values: map[string]interface{}{
			"config": "hello",
			"networkPolicy": map[interface{}]interface{}{"denyCIDR": "10.1.2.3/4"},
			"calico": map[interface{}]interface{}{"networkPolicy": map[interface{}]interface{}{"denyCIDR": "10.5.6.7/8"}},
		},
	}
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
			name: "Values one level deep",
			oldLine: "config: {{ .Values.config }}",
			location: []int{8, 28, 10, 26},
			expected: "config: hello",
			expectedErr: nil,
		},
		{
			name: "Values two levels deep",
			oldLine: "    except: {{ .Values.networkPolicy.denyCIDR }}",
			location: []int{12, 48, 14, 46},
			expected: "    except: 10.1.2.3/4",
			expectedErr: nil,
		},
		{
			name: "Values three levels deep",
			oldLine: "    except: {{ .Values.calico.networkPolicy.denyCIDR }}",
			location: []int{12, 55, 14, 53},
			expected: "    except: 10.5.6.7/8",
			expectedErr: nil,
		},
		{
			name: "Values four levels deep",
			oldLine: "    except: {{ .Values.test.calico.networkPolicy.denyCIDR }}",
			location: []int{12, 60, 14, 58},
			expected: "",
			expectedErr: errors.New("The value .Values.test.calico.networkPolicy.denyCIDR is nested too deeply for this program to currently handle"),
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
