package adminapi

import "testing"

func TestResetTargetIsolation(t *testing.T) {
	for _, tc := range []struct{ prefix, environment, want string }{
		{"", "dev", "attestra-reset-dev"},
		{"attestra-backend-reset-", "next-dev", "attestra-backend-reset-next-dev"},
	} {
		got, err := resetFunctionName(tc.prefix, tc.environment)
		if err != nil || got != tc.want {
			t.Fatalf("%q %v", got, err)
		}
	}
	for _, prefix := range []string{"../../", "arn:aws:lambda:", "other-prefix-"} {
		if _, err := resetFunctionName(prefix, "next-dev"); err == nil {
			t.Fatal("accepted arbitrary prefix")
		}
	}
}
