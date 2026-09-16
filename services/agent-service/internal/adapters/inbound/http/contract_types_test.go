package http_test

import (
	"encoding/json"
	"testing"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

func TestProviderSecretPatchPreservesOmittedNullAndValue(t *testing.T) {
	for _, tc := range []struct {
		name, body, value string
		specified, null   bool
	}{
		{name: "keep", body: `{}`, specified: false},
		{name: "clear", body: `{"api_key":null}`, specified: true, null: true},
		{name: "replace", body: `{"api_key":"test-value"}`, specified: true, value: "test-value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var patch gen.ProviderUpdateRequest
			if err := json.Unmarshal([]byte(tc.body), &patch); err != nil {
				t.Fatal(err)
			}
			if patch.ApiKey.IsSpecified() != tc.specified || patch.ApiKey.IsNull() != tc.null {
				t.Fatal("codegen lost the distinction between omitted, null and set")
			}
			if tc.value != "" {
				value, err := patch.ApiKey.Get()
				if err != nil || value != tc.value {
					t.Fatal("replacement value lost")
				}
			}
		})
	}
}
