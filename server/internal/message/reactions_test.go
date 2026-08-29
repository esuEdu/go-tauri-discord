package message

import "testing"

func TestValidEmojiTakesEmojiAndRefusesText(t *testing.T) {
	accepted := []string{"👍", "❤️", "🎉", "🇧🇷", "👩‍💻", "1️⃣", "✅"}
	for _, emoji := range accepted {
		if err := validEmoji(emoji); err != nil {
			t.Errorf("validEmoji(%q) = %v, want it accepted", emoji, err)
		}
	}

	refused := []string{"", "lol", "👍 ", "a", "👍👎🎉❤️🔥👀✅🎈💯", "\x00", "e"}
	for _, bad := range refused {
		if err := validEmoji(bad); err == nil {
			t.Errorf("validEmoji(%q) accepted it; the column would become free text", bad)
		}
	}
}
