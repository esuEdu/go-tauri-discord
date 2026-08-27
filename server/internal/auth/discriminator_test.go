package auth

import "testing"

func TestADiscriminatorIsNeverTheDeletedUsersOwn(t *testing.T) {
	taken := make([]string, 0, 9998)
	for n := lowestDiscriminator; n <= highestDiscriminator; n++ {
		if format(n) == "0007" {
			continue
		}
		taken = append(taken, format(n))
	}

	got, ok := pickDiscriminator(taken)
	if !ok {
		t.Fatal("one number was free and none was offered")
	}
	if got != "0007" {
		t.Errorf("picked %q, want the only free number 0007", got)
	}
}

func TestANameCanFillUp(t *testing.T) {
	taken := make([]string, 0, highestDiscriminator)
	for n := lowestDiscriminator; n <= highestDiscriminator; n++ {
		taken = append(taken, format(n))
	}

	if got, ok := pickDiscriminator(taken); ok {
		t.Errorf("picked %q from a name with every number taken; the insert would fail with a "+
			"unique violation and the person would be told nothing useful", got)
	}
}

func TestZeroIsHeldBackForTheDeletedUser(t *testing.T) {
	for range 200 {
		got, ok := pickDiscriminator(nil)
		if !ok {
			t.Fatal("nothing was free for a brand new name")
		}
		if got == DiscriminatorDeleted {
			t.Fatalf("handed out %s, which belongs to the placeholder every deleted account is "+
				"reassigned to", DiscriminatorDeleted)
		}
		if len(got) != 4 {
			t.Fatalf("picked %q, want four digits", got)
		}
	}
}

func TestTwoPeopleWithOneNameGetDifferentNumbers(t *testing.T) {
	first, ok := pickDiscriminator(nil)
	if !ok {
		t.Fatal("no number for the first person")
	}
	second, ok := pickDiscriminator([]string{first})
	if !ok {
		t.Fatal("no number for the second person")
	}
	if first == second {
		t.Error("both people got the same number, so the pair that is supposed to be unique is not")
	}
}
