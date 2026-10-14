package domain

import "testing"

func TestSupportsKnowledgeHub(t *testing.T) {
	if SupportsKnowledgeHub(DevServerCapability{Features: []string{"agent.execPrompt", "agent.execPrompt.readonly"}}) {
		t.Fatal("an agent without the feature must not be offered the knowledge root")
	}
	if !SupportsKnowledgeHub(DevServerCapability{Features: []string{"agent.execPrompt.knowledgeHub"}}) {
		t.Fatal("an agent that declares the feature supports it")
	}
	if SupportsKnowledgeHub(DevServerCapability{}) {
		t.Fatal("an unknown capability supports nothing")
	}
}

func TestFeatureKnowledgeHubMatchesTheAgentDeclaration(t *testing.T) {
	if FeatureKnowledgeHub != "agent.execPrompt.knowledgeHub" {
		t.Fatalf("got %q", FeatureKnowledgeHub)
	}
}
