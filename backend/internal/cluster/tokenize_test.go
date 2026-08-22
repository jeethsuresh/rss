package cluster

import "testing"

func TestTokenizeDropsStopwords(t *testing.T) {
	vec := Tokenize("The news of the day", "The president and the senate will meet in the morning.", nil)
	if _, ok := vec["the"]; ok {
		t.Fatal("stopword 'the' should be dropped")
	}
	if _, ok := vec["and"]; ok {
		t.Fatal("stopword 'and' should be dropped")
	}
	if _, ok := vec["of"]; ok {
		t.Fatal("stopword 'of' should be dropped")
	}
}

func TestTitleCapitalizationIsNotProperNoun(t *testing.T) {
	vec := Tokenize("Biden Meets Zelensky In Kyiv", "the talks continue today.", nil)
	if vec["biden"] != 1 {
		t.Fatalf("title Biden should weigh 1, got %v", vec["biden"])
	}
	if vec["zelensky"] != 1 {
		t.Fatalf("title Zelensky should weigh 1, got %v", vec["zelensky"])
	}
}

func TestBodyCapitalizationIsProperNoun(t *testing.T) {
	vec := Tokenize("talks continue", "Biden meets Zelensky in Kyiv today.", nil)
	if vec["biden"] != 3 {
		t.Fatalf("body Biden should weigh 3, got %v", vec["biden"])
	}
	if vec["zelensky"] != 3 {
		t.Fatalf("body Zelensky should weigh 3, got %v", vec["zelensky"])
	}
	if vec["kyiv"] != 3 {
		t.Fatalf("body Kyiv should weigh 3, got %v", vec["kyiv"])
	}
	if vec["meets"] != 1 {
		t.Fatalf("body meets should weigh 1, got %v", vec["meets"])
	}
}

func TestTokenizeIgnoresHTMLAndUsesRSSBody(t *testing.T) {
	vec := Tokenize("", "<p>Apple <b>unveils</b> Vision</p>", nil)
	if vec["apple"] != 3 {
		t.Fatalf("Apple in HTML body should be a proper noun, got %v", vec["apple"])
	}
	if _, ok := vec["p"]; ok {
		t.Fatal("HTML tag names should not be tokens")
	}
}

func TestLearnedMultiplierApplies(t *testing.T) {
	weights := map[string]TokenTally{"biden": {Up: 4, Down: 0}} // 1+0.25*4 = 2
	vec := Tokenize("", "Biden spoke.", weights)
	if vec["biden"] != 6 { // 3 * 2
		t.Fatalf("expected 6, got %v", vec["biden"])
	}
}

func TestLearnedWeightClamps(t *testing.T) {
	if w := learnedMultiplier(TokenTally{Up: 100, Down: 0}); w != 4 {
		t.Fatalf("up clamp %v", w)
	}
	if w := learnedMultiplier(TokenTally{Up: 0, Down: 100}); w != 0.1 {
		t.Fatalf("down clamp %v", w)
	}
}
