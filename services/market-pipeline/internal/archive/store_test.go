package archive

import "testing"

func TestContentHashIsStable(t *testing.T) {
	first := contentHash([]byte(`{"event_id":"one"}`))
	second := contentHash([]byte(`{"event_id":"one"}`))
	different := contentHash([]byte(`{"event_id":"two"}`))
	if first != second {
		t.Fatal("identical content produced different hashes")
	}
	if first == different {
		t.Fatal("different content produced an identical hash")
	}
}

func TestIsNotFoundRejectsOrdinaryErrors(t *testing.T) {
	if isNotFound(assertionError("network unavailable")) {
		t.Fatal("ordinary errors must not be treated as an absent object")
	}
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
