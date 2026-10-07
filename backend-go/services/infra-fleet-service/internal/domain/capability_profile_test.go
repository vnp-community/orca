package domain

import (
	"testing"
)

func TestComputeFingerprint_IgnoresVolatileKeys(t *testing.T) {
	json1 := []byte(`{"agentVersion":"1.0","host":{"os":"linux","memFreeMb":100,"diskFreeMb":100,"loadAvg1":1.5},"probedAt":"2023-01-01"}`)
	json2 := []byte(`{"agentVersion":"1.0","host":{"os":"linux","memFreeMb":200,"diskFreeMb":200,"loadAvg1":2.5},"probedAt":"2023-01-02"}`)

	fp1, err := ComputeFingerprint(json1)
	if err != nil {
		t.Fatalf("ComputeFingerprint: %v", err)
	}
	fp2, err := ComputeFingerprint(json2)
	if err != nil {
		t.Fatalf("ComputeFingerprint: %v", err)
	}

	if fp1 != fp2 {
		t.Errorf("expected fingerprints to match, got %q and %q", fp1, fp2)
	}
}

func TestComputeFingerprint_KeyOrderIndependent(t *testing.T) {
	json1 := []byte(`{"a":1,"b":2}`)
	json2 := []byte(`{"b":2,"a":1}`)

	fp1, _ := ComputeFingerprint(json1)
	fp2, _ := ComputeFingerprint(json2)

	if fp1 != fp2 {
		t.Errorf("expected fingerprints to match for different key orders, got %q and %q", fp1, fp2)
	}
}

func TestComputeFingerprint_DiffersOnToolVersion(t *testing.T) {
	json1 := []byte(`{"tools":{"go":"1.20"}}`)
	json2 := []byte(`{"tools":{"go":"1.21"}}`)

	fp1, _ := ComputeFingerprint(json1)
	fp2, _ := ComputeFingerprint(json2)

	if fp1 == fp2 {
		t.Errorf("expected fingerprints to differ for different tool versions, got %q", fp1)
	}
}

func TestNormalizeFeatures_SortsDedupes(t *testing.T) {
	input := []string{"b", "a", "", "c", "a", "d"}
	got := NormalizeFeatures(input)
	expected := []string{"a", "b", "c", "d"}

	if len(got) != len(expected) {
		t.Fatalf("expected len %d, got %d", len(expected), len(got))
	}
	for i, v := range expected {
		if got[i] != v {
			t.Errorf("at index %d: expected %q, got %q", i, v, got[i])
		}
	}
}

func TestCapabilityProfile_Degraded(t *testing.T) {
	p1 := CapabilityProfile{Source: ProfileSourceProbe}
	if p1.Degraded() {
		t.Errorf("expected Degraded() = false for probe")
	}

	p2 := CapabilityProfile{Source: ProfileSourceHandshakeOnly}
	if !p2.Degraded() {
		t.Errorf("expected Degraded() = true for handshake_only")
	}
}
