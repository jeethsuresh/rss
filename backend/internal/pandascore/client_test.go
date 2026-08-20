package pandascore_test

import (
	"testing"

	"github.com/jeeth/rss-reader/backend/internal/domain"
	"github.com/jeeth/rss-reader/backend/internal/pandascore"
)

func TestMapTier(t *testing.T) {
	cases := map[string]domain.DotaEventTier{
		"s":            domain.DotaTierPremier,
		"a":            domain.DotaTierProfessional,
		"b":            domain.DotaTierSemiPro,
		"c":            domain.DotaTierAmateur,
		"premier":      domain.DotaTierPremier,
		"professional": domain.DotaTierProfessional,
		"":             domain.DotaTierUnknown,
	}
	for in, want := range cases {
		if got := pandascore.MapTier(in); got != want {
			t.Fatalf("MapTier(%q)=%q want %q", in, got, want)
		}
	}
}

func TestMapMatchStatus(t *testing.T) {
	if pandascore.MapMatchStatus("running") != domain.DotaMatchLive {
		t.Fatal("running")
	}
	if pandascore.MapMatchStatus("finished") != domain.DotaMatchCompleted {
		t.Fatal("finished")
	}
	if pandascore.MapMatchStatus("not_started") != domain.DotaMatchUpcoming {
		t.Fatal("not_started")
	}
}
