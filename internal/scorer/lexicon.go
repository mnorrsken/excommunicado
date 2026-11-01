package scorer

import (
	"math"
	"slices"
	"strings"
	"unicode"
)

// Terms are lower case and split into words on anything that is not a letter or
// digit, so "best in class" also matches "best-in-class". A trailing * on the last
// word matches any ending, which covers inflections and most Swedish compounds.
// Words with a common plain meaning (Swedish "resa" = trip) stay out on purpose.

// Strong terms are nearly always corporate speech. Each counts 1.
var strongTerms = []string{
	// English
	"synerg*", "leverag*", "paradigm shift*", "best in class", "world class", "core competenc*",
	"move the needle", "moving the needle", "low hanging fruit", "boil the ocean", "deep dive*",
	"holistic*", "thought leader*", "game chang*", "future proof*", "value chain*", "value add*",
	"value creation", "value proposition*", "customer centric*", "people centric", "purpose driven",
	"results driven", "growth mindset", "transformation journey", "transformational", "empower*",
	"reimagin*", "north star", "right siz*", "rightsiz*", "operational excellence",
	"center of excellence", "centre of excellence", "break down silos", "breaking down silos",
	"co create*", "co creating", "co creation", "lean in", "double down", "doubling down",
	"circle back", "touch base", "touching base", "take this offline", "take it offline",
	"strategic imperative*", "strategic priorit*", "win win", "mindshare", "wheelhouse", "ideat*",
	"cutting edge", "impactful", "actionable insight*", "key takeaway*", "key learnings", "learnings",
	"quick win*", "pain point*", "mission critical", "disruptive", "next generation", "drink the kool aid",
	"innovation at scale", "unlock value", "unlock synerg*", "drive value", "deliver value",
	"ecosystem of", "nimble", "single source of truth", "at the end of the day",
	// Swedish
	"synerg*", "värdeskap*", "kraftsaml*", "helhetsgrepp*", "paradigmskift*", "målbild*",
	"kärnkompeten*", "kärnvärde*", "värdekedj*", "förändringsresa*", "transformationsresa*",
	"tillväxtresa*", "utvecklingsresa*", "digitaliseringsresa*", "hållbarhetsresa*", "resan framåt",
	"resa framåt", "syftesdriv*", "kundcentr*", "framtidssäk*", "samskap*", "medskick*",
	"lyfta blicken", "lyfter blicken", "i världsklass", "världsledande", "operativ excellens",
	"holistisk*", "mindset", "ekosystem*", "innovationsekosystem*", "innovationskraft*",
	"handlingskraft*", "framgångsfaktor*", "nyckelfaktor*", "nyckelbudskap*", "strategiska prioriteringar*",
	"bryta ner silos", "tvärfunktionell*", "snabba vinster", "best practice", "mervärde*", "affärsnytta*",
	"kundresa*", "digital transformation", "tillitsbaser*", "gemensam målbild", "helhetssyn*",
	"tillväxtorienter*", "tillvarata*",
}

// Mild terms are jargon, but also turn up in normal mail. Each counts 0.5.
var mildTerms = []string{
	// English
	"stakeholder*", "align", "aligned", "aligning", "alignment", "sync", "takeaway*", "roadmap*",
	"journey", "ecosystem*", "unlock*", "operating model*", "cross functional*", "silo*", "seamless*",
	"end to end", "at scale", "scalable", "agile", "going forward", "bandwidth", "data driven",
	"actionable", "deliverable*", "best practice*", "innovation", "innovative", "excellence", "robust",
	"state of the art", "strategic", "transformation", "holistically", "proactive*",
	"on the same page", "heads up", "reach out", "reaching out", "hard stop", "onboarding",
	// Swedish
	"agil*", "möjliggör*", "säkerställ*", "intressent*", "förankr*", "samsyn*", "avstämning*",
	"stäm av", "stämma av", "stämmer av", "återkoppl*", "proaktiv*", "silos", "silo", "datadriv*",
	"excellens", "skalbar*", "kundupplevelse*", "strategisk*", "transformation*", "innovation*",
	"förhållningssätt*", "förutsättningar", "verksamhetsmodell*", "avstamp", "kärnverksamhet*",
	"ett grepp", "ta ett samtal", "linjen", "omforma*", "ta höjd", "landa i",
}

type lexTerm struct {
	words  []string
	stem   bool
	weight float64
}

var lexTerms = buildLexicon()

func buildLexicon() []lexTerm {
	var out []lexTerm
	add := func(terms []string, weight float64) {
		for _, t := range terms {
			stem := strings.HasSuffix(t, "*")
			out = append(out, lexTerm{words: words(strings.TrimSuffix(t, "*")), stem: stem, weight: weight})
		}
	}
	add(strongTerms, 1)
	add(mildTerms, 0.5)
	// Longest first, and strong before mild for ties, so "operativ excellens"
	// wins over "excellens".
	slices.SortStableFunc(out, func(a, b lexTerm) int { return len(b.words) - len(a.words) })
	return out
}

func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func (t lexTerm) matches(toks []string) bool {
	if len(toks) < len(t.words) {
		return false
	}
	last := len(t.words) - 1
	for i, w := range t.words[:last] {
		if toks[i] != w {
			return false
		}
	}
	if t.stem {
		return strings.HasPrefix(toks[last], t.words[last])
	}
	return toks[last] == t.words[last]
}

// lexiconScore rates text from 0 to 10 by how dense the known buzzwords are. It
// also returns the phrases it found, each once, in order of appearance.
func lexiconScore(text string) (int, []string) {
	toks := words(text)
	if len(toks) == 0 {
		return 0, nil
	}
	var total float64
	var found []string
	for i := 0; i < len(toks); {
		n := 1
		for _, t := range lexTerms {
			if t.matches(toks[i:]) {
				total += t.weight
				n = len(t.words)
				if p := strings.Join(toks[i:i+n], " "); !slices.Contains(found, p) {
					found = append(found, p)
				}
				break
			}
		}
		i += n
	}
	density := total / float64(len(toks)) * 100
	// The cap on total stops one lone buzzword in a two-line mail from scoring high.
	s := math.Min(math.Min(density*0.5, total*2.5), 10)
	return int(math.Round(s)), found
}
