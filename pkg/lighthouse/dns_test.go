package lighthouse

import (
	"errors"
	"testing"
)

// mockResolver is a test double for Resolver.
type mockResolver struct {
	cname    string
	cnameErr error
	hosts    []string
	hostErr  error
}

func (m *mockResolver) LookupCNAME(_ string) (string, error) {
	return m.cname, m.cnameErr
}

func (m *mockResolver) LookupHost(_ string) ([]string, error) {
	return m.hosts, m.hostErr
}

func TestVerifyDNS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		resolver      *mockResolver
		domain        string
		expectedCNAME string
		expectedIPs   []string
		want          bool
		wantErr       bool
	}{
		{
			name: "matching CNAME",
			resolver: &mockResolver{
				cname: "edge.cloudroof.eu.",
			},
			domain:        "example.com",
			expectedCNAME: "edge.cloudroof.eu",
			want:          true,
		},
		{
			name: "matching CNAME without trailing dot",
			resolver: &mockResolver{
				cname: "edge.cloudroof.eu",
			},
			domain:        "example.com",
			expectedCNAME: "edge.cloudroof.eu",
			want:          true,
		},
		{
			name: "CNAME case insensitive",
			resolver: &mockResolver{
				cname: "EDGE.CLOUDROOF.EU.",
			},
			domain:        "example.com",
			expectedCNAME: "edge.cloudroof.eu",
			want:          true,
		},
		{
			name: "CNAME mismatch falls back to IP match",
			resolver: &mockResolver{
				cname: "other.example.com.",
				hosts: []string{"1.2.3.4"},
			},
			domain:        "example.com",
			expectedCNAME: "edge.cloudroof.eu",
			expectedIPs:   []string{"1.2.3.4"},
			want:          true,
		},
		{
			name: "matching IP in A record",
			resolver: &mockResolver{
				cnameErr: errors.New("no CNAME"),
				hosts:    []string{"1.2.3.4", "5.6.7.8"},
			},
			domain:      "example.com",
			expectedIPs: []string{"1.2.3.4"},
			want:        true,
		},
		{
			name: "no matching CNAME or IP",
			resolver: &mockResolver{
				cname: "wrong.example.com.",
				hosts: []string{"9.9.9.9"},
			},
			domain:        "example.com",
			expectedCNAME: "edge.cloudroof.eu",
			expectedIPs:   []string{"1.2.3.4"},
			want:          false,
		},
		{
			name: "DNS lookup error returns false and error",
			resolver: &mockResolver{
				cnameErr: errors.New("lookup failed"),
				hostErr:  errors.New("lookup failed"),
			},
			domain:      "example.com",
			expectedIPs: []string{"1.2.3.4"},
			want:        false,
			wantErr:     true,
		},
		{
			name: "no expected IPs and CNAME mismatch",
			resolver: &mockResolver{
				cname: "wrong.example.com.",
			},
			domain:        "example.com",
			expectedCNAME: "edge.cloudroof.eu",
			expectedIPs:   nil,
			want:          false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := VerifyDNS(tt.resolver, tt.domain, tt.expectedCNAME, tt.expectedIPs)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyDNS() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("VerifyDNS() = %v, want %v", got, tt.want)
			}
		})
	}
}
