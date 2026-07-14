package acif

import "testing"

const (
	tvSkillG1  = "12d3cb2f53305b53fff6ca4ff1166fd1d063b1319642e0a0906a5539c8fa57b7"
	tvSkillG3  = "fef9a3e615ab4d41c1a990908bf9a3e5b47f70a73eaa16fc7e1578c9f4ab60df"
	tvSkillN   = "b08c2af8a9d5c2eced158b49a13bacfe3d0a22221b965bd2a46672bc5b8f9648"
	tvRuleA    = "3870d9fb5e9cff74b36ffd20141fd693c8500ff5e97469f4b2d41775c242feaf"
	tvRuleK    = "4bcb019272bd88d646c10ca0b7ca088bf8215d0114f08e43ef164884f50fed7c"
	tvCommandA = "eb6f4eb9bc130773a45fb39b20e9ad8e8a05fdabc011d85eeaf47dec08fa5cea"
	tvCommandB = "200dd55e5537f6a8b7458781d1fb8e7e038b1f665a2ebe1eb1aea7aeea19a8ad"
	tvCommandH = "711eb812c3c9e2ee14ae51852e6fe951be85a0141ec514d97eb7d63249df89fa"
	tvMCPStdio = "26387bc7f0b779925f2d6e704f3dfe590fd381893aa301bc28d2ae399f5e3b52"
	tvMCPHTTP  = "e55a9fc091b4e5267c4c07199ee60712de6c47a9846f929c5cf2d1d9da57f98a"
)

func TestRequiresVerdictCanonicalState(t *testing.T) {
	t.Parallel()

	empty := map[string]any{"requires": map[string]any{}}
	if verdict := applyRequiresVerdict(empty); verdict != nil {
		t.Fatalf("empty requires verdict = %#v, want nil", verdict)
	}
	if _, ok := empty["requires"]; ok {
		t.Fatalf("empty requires was not deleted: %#v", empty)
	}

	foreign := map[string]any{"requires": map[string]any{"future": true}}
	verdict := applyRequiresVerdict(foreign)
	if verdict == nil || verdict.Reason != ReasonRequiresOrphanKey {
		t.Fatalf("foreign requires verdict = %#v, want orphan key", verdict)
	}
	if _, ok := foreign["requires"]; !ok {
		t.Fatalf("non-empty requires was removed: %#v", foreign)
	}
}
