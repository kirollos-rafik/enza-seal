package team

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func key(t *testing.T) string {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	return id.Recipient().String()
}

func sample(t *testing.T) (string, map[string]string) {
	k := map[string]string{"alice-l": key(t), "alice-w": key(t), "bob-l": key(t), "carol-l": key(t)}
	js := `{"version":1,"people":{
	 "alice":{"name":"Alice","devices":{"laptop":"` + k["alice-l"] + `","workspace":"` + k["alice-w"] + `"}},
	 "bob":{"name":"Bob","devices":{"laptop":"` + k["bob-l"] + `"}},
	 "carol":{"name":"Carol","devices":{"laptop":"` + k["carol-l"] + `"}},
	 "dan":{"name":"Dan","devices":{}}},
	 "groups":{"pay":["bob","carol"],"all":["alice","bob"]}}`
	return js, k
}

func TestResolve(t *testing.T) {
	js, k := sample(t)
	f, err := Parse([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.Resolve("me", "alice")
	if err != nil || len(got) != 2 {
		t.Fatalf("me: %v %v", got, err)
	}
	got, err = f.Resolve("@pay", "")
	if err != nil || len(got) != 2 || got[0] != k["bob-l"] {
		t.Fatalf("group: %v %v", got, err)
	}
	got, _ = f.Resolve("bob", "")
	if len(got) != 1 || got[0] != k["bob-l"] {
		t.Fatalf("person: %v", got)
	}
}

func TestResolveErrors(t *testing.T) {
	js, _ := sample(t)
	f, _ := Parse([]byte(js))
	for _, spec := range []string{"nobody", "@nogroup", "dan"} {
		if _, err := f.Resolve(spec, ""); err == nil {
			t.Errorf("%q should fail", spec)
		}
	}
	if _, err := f.Resolve("me", ""); err == nil {
		t.Error(`"me" without setup should fail`)
	}
}

func TestParseRejectsBadFiles(t *testing.T) {
	cases := map[string]string{
		"not json":      `{`,
		"bad version":   `{"version":2,"people":{}}`,
		"bad key":       `{"version":1,"people":{"a":{"name":"A","devices":{"l":"age1bogus"}}}}`,
		"unknown group": `{"version":1,"people":{},"groups":{"g":["ghost"]}}`,
	}
	for name, js := range cases {
		if _, err := Parse([]byte(js)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSyncCachesOnlyValidFiles(t *testing.T) {
	js, _ := sample(t)
	bad := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bad {
			_, _ = w.Write([]byte(`{"version":9}`))
			return
		}
		_, _ = w.Write([]byte(js))
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "sub", "recipients.json")
	if _, err := Sync(srv.URL, path); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	bad = true
	if _, err := Sync(srv.URL, path); err == nil {
		t.Fatal("invalid file accepted")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) || !strings.Contains(string(after), "alice") {
		t.Fatal("cache was overwritten by an invalid download")
	}
	if _, err := Sync("", path); err == nil {
		t.Fatal("empty url accepted")
	}
}
