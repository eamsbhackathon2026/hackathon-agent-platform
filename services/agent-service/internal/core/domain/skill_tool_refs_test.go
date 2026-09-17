package domain_test

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestParseSkillToolRefs(t *testing.T) {
	for _, testCase := range []struct {
		name, body string
		want       []string
	}{
		{"two references in prose", "Gọi $get_insights rồi $get_portfolio.", []string{"get_insights", "get_portfolio"}},
		{"repeats collapse", "$get_insights và $get_insights", []string{"get_insights"}},
		{"sorted regardless of order", "$zebra trước $alpha", []string{"alpha", "zebra"}},
		{"money is not a reference", "Chi 5$ và $100 triệu, còn $5tr nữa", nil},
		{"sigil needs a word boundary", "email a$b, biến $$HOME", nil},
		{"mcp tool carries its server", "Dùng $finance.get_portfolio.", []string{"finance.get_portfolio"}},
		{"inline code still counts", "Đọc `$get_insights` cho tháng này.", []string{"get_insights"}},
		{"start of body", "$get_insights mở đầu.", []string{"get_insights"}},
		{"start of line", "Luật:\n$get_insights\n", []string{"get_insights"}},
		{"adjacent references", "$alpha,$beta", []string{"alpha", "beta"}},
		{"hyphenated slug", "Gọi $risk-score.", []string{"risk-score"}},
		{"separator between references stays a boundary", "$alpha-$beta", []string{"alpha", "beta"}},
		{"trailing separator is not part of a name", "Xem $alpha- rồi thôi.", []string{"alpha"}},
		{"underscore is not a boundary", "$alpha_$beta", []string{"alpha"}},
		{"fenced block is an example, not an instruction", "Gọi $get_insights.\n\n```bash\necho $HOME/bin:$PATH\nfor f in $files; do echo $f; done\n```\n\nXong.", []string{"get_insights"}},
		{"tilde fence too", "Gọi $get_insights.\n~~~\n$HOME\n~~~\n", []string{"get_insights"}},
		{"reference after a closed fence still counts", "```\n$HOME\n```\nDùng $get_insights.", []string{"get_insights"}},
		{"indented fence", "  ```\n$HOME\n  ```\nDùng $get_insights.", []string{"get_insights"}},
		{"no references", "Không nhắc công cụ nào.", nil},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got := domain.ParseSkillToolRefs(testCase.body)
			if len(got) == 0 && len(testCase.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("ParseSkillToolRefs(%q) = %v, want %v", testCase.body, got, testCase.want)
			}
		})
	}
}

func TestParseSkillToolRefsReturnsEmptyNotNilShape(t *testing.T) {
	refs := domain.ParseSkillToolRefs("không có gì")
	if refs == nil {
		t.Fatal("expected an empty slice so the JSON contract never sees null")
	}
	if len(refs) != 0 {
		t.Fatalf("expected no references, got %v", refs)
	}
}

func TestValidSkillToolRef(t *testing.T) {
	for _, ref := range []string{"get_insights", "finance.get_portfolio", "risk-score", "A1"} {
		if !domain.ValidSkillToolRef(ref) {
			t.Errorf("ValidSkillToolRef(%q) = false, want true", ref)
		}
	}
	for _, ref := range []string{"", "1tool", "_tool", "a b", "a..b", "tool.", ".tool", "tool-", "tool_", strings.Repeat("a", 65)} {
		if domain.ValidSkillToolRef(ref) {
			t.Errorf("ValidSkillToolRef(%q) = true, want false", ref)
		}
	}
}

func TestValidateSkillRejectsTooManyToolRefs(t *testing.T) {
	refs := make([]string, domain.MaxSkillToolRefs+1)
	for index := range refs {
		refs[index] = "tool_" + strconv.Itoa(index)
	}
	skill := validSkillFixture()
	skill.ToolRefs = refs
	if err := domain.ValidateSkill(skill); err == nil {
		t.Fatal("expected validation error for more than MaxSkillToolRefs references")
	}
}

func TestValidateSkillRejectsMalformedToolRef(t *testing.T) {
	skill := validSkillFixture()
	skill.ToolRefs = []string{"get_insights", "1invalid"}
	if err := domain.ValidateSkill(skill); err == nil {
		t.Fatal("expected validation error for a malformed reference")
	}
}

func TestValidateSkillAcceptsDeclaredToolRefs(t *testing.T) {
	skill := validSkillFixture()
	skill.ToolRefs = []string{"finance.get_portfolio", "get_insights"}
	if err := domain.ValidateSkill(skill); err != nil {
		t.Fatalf("ValidateSkill returned %v, want nil", err)
	}
}

func validSkillFixture() domain.Skill {
	return domain.Skill{
		Name:           "Trợ lý tài chính",
		Description:    "Luật làm việc.",
		SourceType:     domain.SkillSourceMarkdown,
		SourceFilename: "skill.md",
		Content:        "---\nname: Trợ lý tài chính\n---\n\nNội dung.",
		Checksum:       strings.Repeat("a", 64),
	}
}

func TestRewriteSkillToolRefs(t *testing.T) {
	specs := []domain.ToolSpec{
		{Name: "http_get_insights", Ref: "get_insights"},
		{Name: "mcp_finance_get_portfolio", Ref: "finance.get_portfolio"},
		{Name: "http_risk_score_a1b2c3", Ref: "risk-score"},
		{Name: "no_ref_tool"},
	}
	for _, testCase := range []struct {
		name, body, want string
	}{
		{"declared tool takes the run-time name", "Gọi $get_insights.", "Gọi http_get_insights."},
		{"mcp reference resolves too", "Dùng $finance.get_portfolio.", "Dùng mcp_finance_get_portfolio."},
		{"disambiguating suffix carries through", "Xem $risk-score.", "Xem http_risk_score_a1b2c3."},
		{"unbound reference keeps a bare name", "Gọi $get_portfolio.", "Gọi get_portfolio."},
		{"separator between references survives", "$get_insights-$get_insights", "http_get_insights-http_get_insights"},
		{"money is untouched", "Chi 5$ và $100 triệu.", "Chi 5$ và $100 triệu."},
		{"fenced example is untouched", "Gọi $get_insights.\n```bash\necho $get_insights\n```", "Gọi http_get_insights.\n```bash\necho $get_insights\n```"},
		{"line structure survives", "Một $get_insights\nHai $finance.get_portfolio\n", "Một http_get_insights\nHai mcp_finance_get_portfolio\n"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := domain.RewriteSkillToolRefs(testCase.body, specs); got != testCase.want {
				t.Fatalf("RewriteSkillToolRefs(%q) = %q, want %q", testCase.body, got, testCase.want)
			}
		})
	}
}

func TestRewriteSkillToolRefsWithoutToolsStripsSigil(t *testing.T) {
	if got := domain.RewriteSkillToolRefs("Gọi $get_insights.", nil); got != "Gọi get_insights." {
		t.Fatalf("got %q", got)
	}
}
