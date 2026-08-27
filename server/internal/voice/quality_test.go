package voice

import (
	"testing"
	"time"
)

func TestGradingSaysWhatSomebodyWouldNotice(t *testing.T) {
	cases := []struct {
		name string
		loss float64
		rtt  time.Duration
		want string
	}{
		{"a clean link", 0, 30 * time.Millisecond, QualityGood},
		{"loss you can hear", 0.03, 40 * time.Millisecond, QualityFair},
		{"loss that breaks words up", 0.12, 40 * time.Millisecond, QualityPoor},
		{"a slow link with no loss", 0, 250 * time.Millisecond, QualityFair},
		{"far enough away to talk over each other", 0, 600 * time.Millisecond, QualityPoor},
		{"bad on both counts is still just bad", 0.2, 900 * time.Millisecond, QualityPoor},
	}

	for _, c := range cases {
		if got := grade(c.loss, c.rtt); got != c.want {
			t.Errorf("%s: grade(%.2f, %s) = %s, want %s", c.name, c.loss, c.rtt, got, c.want)
		}
	}
}
