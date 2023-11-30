package session

import (
 	"math"
	"testing"
	"fmt"

	"github.com/ipfs/boxo/bitswap/internal/testutil"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

func TestPeerResponseTrackerInit(t *testing.T) {
	peers := testutil.GeneratePeers(2)
	prt := newPeerResponseTracker()

	if prt.choose([]peer.ID{}) != "" {
		t.Fatal("expected empty peer ID")
	}
	if prt.choose([]peer.ID{peers[0]}) != peers[0] {
		t.Fatal("expected single peer ID")
	}
	p := prt.choose(peers)
	if p != peers[0] && p != peers[1] {
		t.Fatal("expected randomly chosen peer")
	}
} 

func TestPeerResponseTrackerProbabilityUnknownPeers(t *testing.T) {
	peers := testutil.GeneratePeers(4)
	prt := newPeerResponseTracker()

	choices := []int{0, 0, 0, 0}
	count := 1000
	for i := 0; i < count; i++ {
		p := prt.choose(peers)
		if p == peers[0] {
			choices[0]++
		} else if p == peers[1] {
			choices[1]++
		} else if p == peers[2] {
			choices[2]++
		} else if p == peers[3] {
			choices[3]++
		}
	}

	for _, c := range choices {
		
		if c == 0 {
			t.Fatal("expected each peer to be chosen at least once")
		}
		if math.Abs(float64(c-choices[0])) > 0.2*float64(count) {
			t.Fatal("expected unknown peers to have roughly equal chance of being chosen")
		}
	}
}

func TestPeerResponseTrackerProbabilityOneKnownOneUnknownPeer(t *testing.T) {
	peers := testutil.GeneratePeers(2)
	prt := newPeerResponseTracker()

	prt.receivedWantHaveResponse(peers[0], 10)
	prt.receivedBlockFrom(peers[0], 10)

	chooseFirst := 0
	chooseSecond := 0
	for i := 0; i < 1000; i++ {
		p := prt.choose(peers)
		if p == peers[0] {
			chooseFirst++
		} else if p == peers[1] {
			chooseSecond++
		}
	}

	fmt.Println("chooseFirst: ", chooseFirst, ", chooseSecond: ", chooseSecond)

	if chooseSecond == 0 {
		t.Fatal("expected unknown peer to occasionally be chosen")
	}
	if chooseSecond > chooseFirst {
		t.Fatal("expected known peer to be chosen more often")
	}
}

func TestPeerResponseTrackerProbabilityProportional(t *testing.T) {
	peerCount := 3
	peers := testutil.GeneratePeers(peerCount)
	prt := newPeerResponseTracker()

	wantHaveResponseDurations := []int64{10, 60, 30}
	for pi, duration := range wantHaveResponseDurations {
		prt.receivedWantHaveResponse(peers[pi], duration)
	}

	avgBlockResponseDuration := []int64{10, 60, 30}
	receivedBlockCount := 3
	for pi, duration := range avgBlockResponseDuration {
		for i := 0; i < receivedBlockCount; i++ {
			prt.receivedBlockFrom(peers[pi], duration)
		}
	}

	var choices []int
	for range wantHaveResponseDurations {
		choices = append(choices, 0)
	}

	chooseCount := 1000
	for i := 0; i < chooseCount; i++ {
		p := prt.choose(peers)
		if p == peers[0] {
			choices[0]++
		} else if p == peers[1] {
			choices[1]++
		} else if p == peers[2] {
			choices[2]++
		}
	}

	peerValues := []float64{0.667, 0.110, 0.221} // normalized values

	for i, c := range choices {

		fmt.Println("Peer: ", i, ", Amount: ", c)

		if c == 0 {
			t.Fatal("expected each peer to be chosen at least once")
		}


		if math.Abs(float64(c)-(float64(chooseCount)*peerValues[i])) > 0.2*float64(chooseCount) {
			t.Fatal("expected peers to be chosen proportionally to peer value")
		}
	}
}

func TestPeerResponseTrackerProbabilityProportional_For_Different_WantHaveResponse_And_BlockResponse_Duration(t *testing.T) {
	peerCount := 3
	peers := testutil.GeneratePeers(peerCount)
	prt := newPeerResponseTracker()

	lastWantHaveResponseDurations := []int64{10, 60, 30}
	for pi, duration := range lastWantHaveResponseDurations {
		prt.receivedWantHaveResponse(peers[pi], duration)
	}

	avgBlockResponseDuration := []int64{30, 60, 10}
	receivedBlockCount := 3
	for pi, duration := range avgBlockResponseDuration {
		for i := 0; i < receivedBlockCount; i++ {
			prt.receivedBlockFrom(peers[pi], duration)
		}
	}

	var choices []int
	for range lastWantHaveResponseDurations {
		choices = append(choices, 0)
	}

	chooseCount := 1000
	for i := 0; i < chooseCount; i++ {
		p := prt.choose(peers)
		if p == peers[0] {
			choices[0]++
		} else if p == peers[1] {
			choices[1]++
		} else if p == peers[2] {
			choices[2]++
		}
	}

	peerValues := []float64{0.42857, 0.1428, 0.42857} // normalized values

	for i, c := range choices {

		// TODO: fmt.Println("Peer: ", i, ", Amount: ", c)

		if c == 0 {
			t.Fatal("expected each peer to be chosen at least once")
		}

		if math.Abs(float64(c)-(float64(chooseCount)*peerValues[i])) > 0.2*float64(chooseCount) {
			t.Fatal("expected peers to be chosen proportionally to peer value")
		}
	}
}
