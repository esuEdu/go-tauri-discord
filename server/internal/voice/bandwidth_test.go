package voice

import (
	"testing"

	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
)

func TestTheEstimatorMeasuresWithoutHoldingPacketsBack(t *testing.T) {
	estimator, err := newEstimator()
	if err != nil {
		t.Fatalf("new estimator: %v", err)
	}
	t.Cleanup(func() { estimator.Close() })

	const ssrc = 4321
	forwarded := 0
	out := estimator.AddStream(
		&interceptor.StreamInfo{SSRC: ssrc},
		interceptor.RTPWriterFunc(func(*rtp.Header, []byte, interceptor.Attributes) (int, error) {
			forwarded++
			return 0, nil
		}),
	)

	const burst = 400
	for range burst {
		if _, err := out.Write(&rtp.Header{SSRC: ssrc}, make([]byte, 1200), nil); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	if forwarded != burst {
		t.Fatalf("%d of %d packets had left by the time the writes returned; the rest are sitting "+
			"in a pacer queue draining at the current estimate. A screen share metered that way "+
			"falls further behind every second the estimate is below what the publisher sends, "+
			"which is delay this server invented rather than measured", forwarded, burst)
	}
}

func TestJoinAttachesABandwidthEstimator(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID, userID := uuid.New(), uuid.New()
	if err := sfu.Join(channelID, userID, true); err != nil {
		t.Fatalf("join: %v", err)
	}

	estimates := sfu.Estimates(channelID)
	if len(estimates) != 1 {
		t.Fatalf("Estimates returned %d entries, want 1; with none there is no record of what a "+
			"viewer can actually take, which is the whole point of measuring first", len(estimates))
	}
	if estimates[0].UserID != userID {
		t.Errorf("estimate belongs to %s, want %s", estimates[0].UserID, userID)
	}
	if estimates[0].Bits <= 0 {
		t.Errorf("estimate = %d bps, want at least the initial %d", estimates[0].Bits, initialEstimate)
	}
}

func TestEveryPeerGetsAnEstimatorOfItsOwn(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID := uuid.New()
	first, second := uuid.New(), uuid.New()
	for _, userID := range []uuid.UUID{first, second} {
		if err := sfu.Join(channelID, userID, true); err != nil {
			t.Fatalf("join %s: %v", userID, err)
		}
	}

	if got := len(sfu.Estimates(channelID)); got != 2 {
		t.Fatalf("Estimates returned %d entries for two members, want 2", got)
	}

	sfu.mu.Lock()
	defer sfu.mu.Unlock()
	room := sfu.rooms[channelID]
	if room.peers[first].estimate == room.peers[second].estimate {
		t.Error("two members share one estimator, so every reading is the sum of both links " +
			"rather than either of them; the estimator is paired with the connection that was " +
			"just built, and that pairing has come apart")
	}
}

func TestViewerBandwidthLeavesTheSharerOut(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	channelID := uuid.New()
	sharer, viewer := uuid.New(), uuid.New()
	for _, userID := range []uuid.UUID{sharer, viewer} {
		if err := sfu.Join(channelID, userID, true); err != nil {
			t.Fatalf("join %s: %v", userID, err)
		}
	}

	if got := sfu.sharingChannels(); len(got) != 0 {
		t.Fatalf("a channel with nobody sharing was reported as sharing: %v", got)
	}

	sfu.mu.Lock()
	sfu.rooms[channelID].screens[sharer] = "screen"
	sfu.mu.Unlock()

	if got := sfu.sharingChannels(); len(got) != 1 || got[0] != channelID {
		t.Fatalf("sharing channels = %v, want just %s; this is the gate on the whole "+
			"measurement, and nothing is recorded at all when it is wrong", got, channelID)
	}

	summary := sfu.viewerBandwidth(channelID)
	if summary.Viewers != 1 {
		t.Fatalf("counted %d viewers of a share watched by one person; the sharer's own link "+
			"says nothing about what anybody watching can take", summary.Viewers)
	}
	if summary.Lowest != summary.Highest {
		t.Errorf("one viewer reported a spread: lowest %d, highest %d",
			summary.Lowest, summary.Highest)
	}
	if summary.Lowest <= 0 {
		t.Errorf("lowest estimate = %d bps, want at least the initial %d",
			summary.Lowest, initialEstimate)
	}
	if got := summary.spread(); got != 1 {
		t.Errorf("spread across one viewer = %v, want 1", got)
	}
}

func TestSpreadOfNoViewersIsNotADivisionByZero(t *testing.T) {
	if got := (shareBandwidth{}).spread(); got != 0 {
		t.Errorf("spread with nothing measured = %v, want 0", got)
	}
}

func TestEstimatesOfAnEmptyChannelAreNothingRatherThanZero(t *testing.T) {
	sfu, err := New(newRecordingSignaler(), nil)
	if err != nil {
		t.Fatalf("new sfu: %v", err)
	}
	t.Cleanup(sfu.Close)

	if got := sfu.Estimates(uuid.New()); got != nil {
		t.Errorf("Estimates of a channel nobody is in = %v, want nil", got)
	}
}
