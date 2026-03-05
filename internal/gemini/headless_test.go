package gemini

import "testing"

func TestBuildHeadlessArgs(t *testing.T) {
	args := BuildHeadlessArgs("hello", "", false)
	want := []string{"--output-format", "json", "-e", "none", "-p", "hello"}
	if len(args) != len(want) {
		t.Fatalf("len mismatch: got %d want %d", len(args), len(want))
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d] mismatch: got %q want %q", i, args[i], want[i])
		}
	}
}

func TestParseSingleJSONObject(t *testing.T) {
	_, err := ParseSingleJSONObject([]byte(`{"response":"ok","stats":{}}`))
	if err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}

	_, err = ParseSingleJSONObject([]byte("{\"response\":\"ok\"}\nnoise"))
	if err == nil {
		t.Fatal("expected trailing content error")
	}
}

func TestBuildHeadlessArgs_Yolo(t *testing.T) {
	args := BuildHeadlessArgs("hello", "", true)
	want := []string{"--output-format", "json", "-e", "none", "-y", "-p", "hello"}
	if len(args) != len(want) {
		t.Fatalf("len mismatch: got %d want %d", len(args), len(want))
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d] mismatch: got %q want %q", i, args[i], want[i])
		}
	}
}
