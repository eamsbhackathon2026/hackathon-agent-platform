package domain

import (
	"regexp"
	"sort"
	"strings"
)

// MaxSkillToolRefs bounds how many tools one skill may declare. A skill that names a
// hundred tools has stopped being a set of working rules, and an assistant is capped
// at twenty skills anyway, so this ceiling only has to stop a runaway document.
const MaxSkillToolRefs = 100

// A skill describes how to work but nothing in it said which tools that work depends
// on, so removing a tool broke a skill silently. Writing the dependency inline with a
// $ sigil keeps the declaration next to the rule that needs it, with no separate list
// to fall out of sync.
//
// A name starts with a letter, which keeps prices like $100 out, and ends with a
// letter or digit, so the trailing separator in "$alpha-$beta" stays available as the
// word boundary the second reference needs. An optional dotted half names an MCP tool
// as <server slug>.<tool name>.
const (
	skillToolRefHead = `[A-Za-z](?:[A-Za-z0-9_-]{0,62}[A-Za-z0-9])?`
	skillToolRefTail = `[A-Za-z0-9](?:[A-Za-z0-9_-]{0,62}[A-Za-z0-9])?`
	skillToolRefBody = skillToolRefHead + `(?:\.` + skillToolRefTail + `)?`
)

// The leading group captures the character before the sigil so that a$b and $$x are
// skipped: a reference only counts at a word boundary.
var skillToolRefPattern = regexp.MustCompile(`(^|[^A-Za-z0-9_$])\$(` + skillToolRefBody + `)`)

// skillToolRefShape matches one reference on its own, without the surrounding text the
// parser needs, so a value read back from storage can be revalidated.
var skillToolRefShape = regexp.MustCompile(`^` + skillToolRefBody + `$`)

// codeFenceLine opens or closes a fenced block. Everything inside a fence is an example
// the author is showing, not an instruction the assistant follows: a shell snippet is
// full of $variables that name no tool, and rewriting them at run time would corrupt
// the example. Inline backticks still count, because `$get_insights` inside a sentence
// is the natural way to write a reference.
var codeFenceLine = regexp.MustCompile("^[ \t]{0,3}(?:```|~~~)")

// ParseSkillToolRefs reads the tools a skill body declares with a $ sigil, sorted and
// deduplicated so the stored list is stable across imports of the same document.
func ParseSkillToolRefs(body string) []string {
	seen := map[string]bool{}
	refs := []string{}
	mapSkillProse(body, func(line string) string {
		for _, match := range skillToolRefPattern.FindAllStringSubmatch(line, -1) {
			if ref := match[2]; !seen[ref] {
				seen[ref] = true
				refs = append(refs, ref)
			}
		}
		return line
	})
	sort.Strings(refs)
	return refs
}

// ValidSkillToolRef reports whether one reference has a shape the parser could produce.
func ValidSkillToolRef(ref string) bool {
	return skillToolRefShape.MatchString(ref)
}

// mapSkillProse rewrites every line outside a fenced code block and leaves fenced lines
// untouched, so parsing and rewriting agree on which text is instruction and which is
// an example.
func mapSkillProse(body string, rewrite func(string) string) string {
	lines := strings.Split(body, "\n")
	fenced := false
	for index, line := range lines {
		if codeFenceLine.MatchString(line) {
			fenced = !fenced
			continue
		}
		if !fenced {
			lines[index] = rewrite(line)
		}
	}
	return strings.Join(lines, "\n")
}

// skillToolRefsValid checks the reference list carried alongside a skill document.
func skillToolRefsValid(refs []string) bool {
	for _, ref := range refs {
		if !ValidSkillToolRef(ref) {
			return false
		}
	}
	return true
}

// RewriteSkillToolRefs turns the $refs a skill declares into the tool names this run
// actually offers the model, so the instructions name the same tool the model was handed
// a schema for. A reference the run has no tool for loses its sigil and keeps its bare
// name: the capability section is what tells the model the action is unavailable, and a
// stray $ would only read as noise.
func RewriteSkillToolRefs(body string, specs []ToolSpec) string {
	names := make(map[string]string, len(specs))
	for _, spec := range specs {
		if spec.Ref != "" {
			names[spec.Ref] = spec.Name
		}
	}
	return mapSkillProse(body, func(line string) string {
		return skillToolRefPattern.ReplaceAllStringFunc(line, func(match string) string {
			parts := skillToolRefPattern.FindStringSubmatch(match)
			boundary, ref := parts[1], parts[2]
			if name, found := names[ref]; found {
				return boundary + name
			}
			return boundary + ref
		})
	})
}
