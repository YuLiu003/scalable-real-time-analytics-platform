package access

import (
	"os"
	"strings"
	"testing"
)

func TestLoadFailsClosedForNonDemoData(t *testing.T) {
	if token, err := Load("demo", ""); err != nil || token != "" {
		t.Fatalf("demo Load() = %q, %v", token, err)
	}
	if _, err := Load("private", ""); err == nil {
		t.Fatal("unprotected private portfolio was accepted")
	}
}

func TestLoadValidatesPrivateTokenFile(t *testing.T) {
	if _, err := Load("private", "token.txt"); err == nil {
		t.Fatal("relative access token path was accepted")
	}
	missing := t.TempDir() + "/missing"
	if _, err := Load("private", missing); err == nil {
		t.Fatal("missing access token file was accepted")
	} else if strings.Contains(err.Error(), missing) {
		t.Fatal("missing access token error exposed its private path")
	}

	for _, test := range []struct {
		name    string
		content []byte
		want    string
		wantErr bool
	}{
		{name: "minimum length", content: []byte(strings.Repeat("a", 32)), want: strings.Repeat("a", 32)},
		{name: "token68 padding", content: []byte(strings.Repeat("a", 30) + "=="), want: strings.Repeat("a", 30) + "=="},
		{name: "single trailing newline", content: []byte(strings.Repeat("b", 32) + "\n"), want: strings.Repeat("b", 32)},
		{name: "maximum length", content: []byte(strings.Repeat("c", 512)), want: strings.Repeat("c", 512)},
		{name: "too short", content: []byte(strings.Repeat("d", 31)), wantErr: true},
		{name: "too long", content: []byte(strings.Repeat("e", 513)), wantErr: true},
		{name: "space", content: []byte(strings.Repeat("f", 31) + " "), wantErr: true},
		{name: "unicode whitespace", content: []byte(strings.Repeat("f", 31) + "\u00a0"), wantErr: true},
		{name: "control", content: []byte(strings.Repeat("g", 31) + "\x00"), wantErr: true},
		{name: "two trailing newlines", content: []byte(strings.Repeat("h", 32) + "\n\n"), wantErr: true},
		{name: "windows newline", content: []byte(strings.Repeat("i", 32) + "\r\n"), wantErr: true},
		{name: "invalid UTF-8", content: append([]byte(strings.Repeat("j", 31)), 0xff), wantErr: true},
		{name: "non-ASCII", content: []byte(strings.Repeat("é", 32)), wantErr: true},
		{name: "non-token punctuation", content: []byte(strings.Repeat("!", 32)), wantErr: true},
		{name: "padding in middle", content: []byte(strings.Repeat("k", 31) + "=k"), wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := t.TempDir() + "/access-token"
			if err := os.WriteFile(path, test.content, 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := Load("private", path)
			if test.wantErr {
				if err == nil {
					t.Fatalf("Load() = %q, want error", got)
				}
				if content := string(test.content); content != "" && strings.Contains(err.Error(), content) {
					t.Fatal("access token error exposed file contents")
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("Load() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}
