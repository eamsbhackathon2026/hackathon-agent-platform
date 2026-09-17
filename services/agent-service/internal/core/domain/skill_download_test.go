package domain

import "testing"

func TestSkillDownloadFilename(t *testing.T) {
	tests := map[string]struct {
		skill Skill
		want  string
	}{
		"kept upload":         {Skill{SourceType: SkillSourceZIP, SourceFilename: "pack.zip", SourceFileAvailable: true}, "pack.zip"},
		"markdown upload":     {Skill{SourceType: SkillSourceMarkdown, SourceFilename: "guide.markdown"}, "guide.markdown"},
		"zip without upload":  {Skill{SourceType: SkillSourceZIP, SourceFilename: "Trợ lý.zip"}, "Trợ lý.md"},
		"stem falls back":     {Skill{Name: "Review", SourceType: SkillSourceZIP, SourceFilename: ".zip"}, "Review.md"},
		"odd markdown source": {Skill{SourceType: SkillSourceMarkdown, SourceFilename: "notes.txt"}, "notes.md"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := SkillDownloadFilename(test.skill); got != test.want {
				t.Fatalf("got %q want %q", got, test.want)
			}
		})
	}
}
