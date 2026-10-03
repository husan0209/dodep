package shared

import "testing"

// TestReExports pins the root-package re-export surface (lib.go).
// It guards the generic alias/wrapper fixes: this package must compile
// and every re-exported symbol must work.
func TestReExports(t *testing.T) {
	if GenerateUUID() == "" {
		t.Fatal("GenerateUUID must return non-empty id")
	}
	m, err := ParseMoney("100.00", "USD")
	if err != nil {
		t.Fatalf("ParseMoney failed: %v", err)
	}
	if got := FormatMoney(m, "en"); got != "$100.00" {
		t.Fatalf("bad FormatMoney: %s", got)
	}
	if !IsValidEmail("user@example.com") || IsValidEmail("nope") {
		t.Fatal("IsValidEmail re-export broken")
	}
	if !IsValidUUID(GenerateUUID()) {
		t.Fatal("IsValidUUID re-export broken")
	}
	if NewValidationError("x").Code != "VALIDATION_ERROR" {
		t.Fatal("NewValidationError re-export broken")
	}
	if len(Currencies) == 0 || RateLimits.LoginAttempts == 0 {
		t.Fatal("constants re-export broken")
	}
	type payload struct {
		N int `json:"n"`
	}
	cloned, err := DeepClone(payload{N: 7})
	if err != nil || cloned.N != 7 {
		t.Fatalf("DeepClone wrapper broken: %+v %v", cloned, err)
	}
	var zero Money
	if zero.String() != "0 USD" && zero.Amount.String() != "0" {
		t.Fatalf("Money alias broken: %+v", zero)
	}
	var page PaginationResult[string]
	page.Items = []string{"a"}
	if len(page.Items) != 1 {
		t.Fatal("PaginationResult re-export broken")
	}
	var resp ApiResponse[string]
	resp.Data = &page.Items[0]
	if *resp.Data != "a" {
		t.Fatal("ApiResponse re-export broken")
	}
}
