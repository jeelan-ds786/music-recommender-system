package topics

import (
	"reflect"
	"testing"
)

func TestDefinitionsAreUniqueAndExplicit(t *testing.T) {
	seen := make(map[string]bool, len(Definitions))
	for _, definition := range Definitions {
		if definition.Name == "" || definition.Partitions <= 0 || definition.Retention <= 0 || definition.Key == "" || definition.Schema != 1 || definition.Producer == "" || definition.Consumers == "" {
			t.Fatalf("incomplete topic definition: %+v", definition)
		}
		if seen[definition.Name] {
			t.Fatalf("duplicate topic definition %q", definition.Name)
		}
		seen[definition.Name] = true
	}
}

func TestTopicContractNamesKeysAndHeaders(t *testing.T) {
	expected := map[string]struct {
		partitions int
		key        string
		headers    []string
	}{
		"user.registered":            {partitions: 1, key: "user_id"},
		"user.preference.updated":    {partitions: 1, key: "user_id"},
		"user.playlist.updated":      {partitions: 1, key: "user_id"},
		"playback.event.v1":          {partitions: 3, key: "user_id", headers: []string{HeaderContentType, HeaderEventType, HeaderSchemaVersion}},
		"recommendation.feedback.v1": {partitions: 3, key: "user_id", headers: []string{HeaderContentType, HeaderEventType, HeaderSchemaVersion}},
		"event.retry.v1":             {partitions: 3, key: "original key", headers: []string{HeaderContentType, HeaderSchemaVersion}},
		"event.dlq.v1":               {partitions: 1, key: "original key", headers: []string{HeaderContentType, HeaderSchemaVersion}},
	}

	if len(Definitions) != len(expected) {
		t.Fatalf("topic definitions = %d, want %d", len(Definitions), len(expected))
	}
	for _, definition := range Definitions {
		want, ok := expected[definition.Name]
		if !ok {
			t.Fatalf("unexpected topic %q", definition.Name)
		}
		if definition.Partitions != want.partitions || definition.Key != want.key || !reflect.DeepEqual(definition.Headers, want.headers) {
			t.Fatalf("topic %q contract = %+v", definition.Name, definition)
		}
	}
}
