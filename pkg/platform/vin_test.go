package platform

import "testing"

func TestValidateVIN(t *testing.T) {
	if err := ValidateVIN("LSVAA4182E2123456"); err != nil {
		t.Fatal(err)
	}
	cases := []string{"LSVAA4182E212345", "LSVAA4182E21234567", "LSVAA4182E212345I", "lsvaa4182e2123456"}
	for _, vin := range cases {
		if err := ValidateVIN(vin); err == nil {
			t.Fatalf("expected error for %s", vin)
		}
	}
}
