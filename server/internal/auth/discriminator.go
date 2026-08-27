package auth

import (
	"fmt"
	"math/rand/v2"
)

const (
	DiscriminatorDeleted = "0000"

	lowestDiscriminator  = 1
	highestDiscriminator = 9999
	randomAttempts       = 8
)

func pickDiscriminator(taken []string) (string, bool) {
	used := make(map[string]bool, len(taken)+1)
	used[DiscriminatorDeleted] = true
	for _, t := range taken {
		used[t] = true
	}

	for range randomAttempts {
		candidate := format(rand.IntN(highestDiscriminator) + lowestDiscriminator)
		if !used[candidate] {
			return candidate, true
		}
	}

	free := make([]string, 0, highestDiscriminator)
	for n := lowestDiscriminator; n <= highestDiscriminator; n++ {
		if candidate := format(n); !used[candidate] {
			free = append(free, candidate)
		}
	}
	if len(free) == 0 {
		return "", false
	}
	return free[rand.IntN(len(free))], true
}

func format(n int) string {
	return fmt.Sprintf("%04d", n)
}
