package config

import (
	"strings"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	for k, v := range map[string]string{"PORT": "8080", "HOST": "127.0.0.1", "DATABASE_PATH": "./test.db", "SESSION_SECRET": strings.Repeat("x", 32), "COOKIE_SECURE": "false", "PUBLIC_URL": "http://localhost:8080"} {
		t.Setenv(k, v)
	}
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ k, v string }{{"PORT", "70000"}, {"SESSION_SECRET", "short"}, {"DATABASE_PATH", ""}, {"PUBLIC_URL", "javascript:bad"}, {"PUBLIC_URL", "https://site.test/path"}, {"COOKIE_SECURE", "wat"}, {"PUBLIC_URL", "https://site.test"}} {
		t.Run(tc.k+tc.v, func(t *testing.T) {
			t.Setenv(tc.k, tc.v)
			if _, err := Load(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
