package http_test

import (
	"net/url"
	"testing"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

func TestCatalogResourcePagination(t *testing.T) {
	f := newCatalogFixture(t)
	p := f.provider(t)
	owner := identityBearer("owner")
	f.request(t, "POST", "/v1/providers", `{"name":"Second","kind":"gemini","api_key":"fake-secret"}`, owner, 201)
	f.agent(t, p.Id)
	f.request(t, "POST", "/v1/agents", `{"name":"Second","provider_id":"`+p.Id.String()+`","model":"m","system_prompt":"Help"}`, owner, 201)
	for _, resource := range []string{"providers", "agents"} {
		t.Run(resource, func(t *testing.T) {
			path := "/v1/" + resource + "?limit=1"
			type page struct {
				Items []struct {
					ID string `json:"id"`
				} `json:"items"`
				NextCursor *string `json:"next_cursor"`
			}
			first := decodeIdentity[page](t, f.request(t, "GET", path, "", owner, 200))
			if len(first.Items) != 1 || first.NextCursor == nil || *first.NextCursor == "" {
				t.Fatal("missing first page cursor")
			}
			last := decodeIdentity[page](t, f.request(t, "GET", path+"&cursor="+url.QueryEscape(*first.NextCursor), "", owner, 200))
			if len(last.Items) != 1 || last.NextCursor != nil || first.Items[0].ID == last.Items[0].ID {
				t.Fatal("pagination repeats or skips records")
			}
		})
	}
	missing := "00000000-0000-4000-8000-000000000099"
	r := f.request(t, "POST", "/v1/agents", `{"name":"Missing provider","provider_id":"`+missing+`","model":"m","system_prompt":"Help"}`, owner, 404)
	if decodeIdentity[gen.Problem](t, r).Code != gen.ProblemCode("not_found") {
		t.Fatal("missing provider must be actionable")
	}
}
