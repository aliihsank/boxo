package session

import (
	"fmt"
	"math/rand"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

// peerResponseTracker keeps track of how many times each peer was the first
// to send us a block for a given CID (used to rank peers)
type peerResponseTracker struct {
	firstResponder map[peer.ID]int
	lastHaveResponseDuration map[peer.ID]int64
	avgBlockResponseDuration map[peer.ID]int64
	blockResponseCount map[peer.ID]int64
}

func newPeerResponseTracker() *peerResponseTracker {
	return &peerResponseTracker{
		firstResponder: make(map[peer.ID]int),
		lastHaveResponseDuration: make(map[peer.ID]int64),
		avgBlockResponseDuration: make(map[peer.ID]int64),
		blockResponseCount: make(map[peer.ID]int64),
	}
}

func (prt *peerResponseTracker) receivedWantHaveResponse(from peer.ID, responseDuration int64) {
	prt.lastHaveResponseDuration[from] = responseDuration
}

// receivedBlockFrom is called when a block is received from a peer
// (only called first time block is received)
func (prt *peerResponseTracker) receivedBlockFrom(from peer.ID, responseDuration int64) {
	prt.firstResponder[from]++

	totalResponseDuration := prt.avgBlockResponseDuration[from] * prt.blockResponseCount[from]
	totalResponseDuration += responseDuration
	
	prt.blockResponseCount[from]++

	prt.avgBlockResponseDuration[from] = totalResponseDuration / prt.blockResponseCount[from]

	fmt.Println("Received Block response from: ", from, ", Duration: ", responseDuration)
}

// choose picks a peer from the list of candidate peers, favouring those peers
// that were first to send us previous blocks
func (prt *peerResponseTracker) choose(peers []peer.ID) peer.ID {
	if len(peers) == 0 {
		return ""
	}

	rnd := rand.Float64()

	// Find the total received blocks for all candidate peers
	var total float64 = 0
	for _, p := range peers {
		peerVal := prt.getPeerValue(p)
		total += peerVal
	}
	
	// Choose one of the peers with a chance proportional to the number
	// of blocks received from that peer
	counted := 0.0
	fmt.Println("Choosing best peer...")
	for _, p := range peers {
		peerVal := prt.getPeerValue(p)
		peerProbability := float64(peerVal) / float64(total)
		
		fmt.Println("Peer: ", p, ", Peer Val: ", peerVal, ", Total: ", total, ", Peer Probability: ", peerProbability, ", Cumulative: ", counted + peerProbability, ", Threshold: ", rnd)
		counted += peerProbability
		if counted > rnd {
			fmt.Println("Chose Peer: ", p, " as best peer.")
			return p
		}
	}

	// We shouldn't get here unless there is some weirdness with floating point
	// math that doesn't quite cover the whole range of peers in the for loop
	// so just choose the last peer.
	index := len(peers) - 1
	return peers[index]
}

func (prt *peerResponseTracker) getPeerValue(p peer.ID) float64 {
	// Make sure there is always at least a small chance a new peer
	// will be chosen

	// TODO: a + b = 1, a > b
	a := 0.3
	b := 0.7

	lastWantHaveResponseTime := prt.lastWantHaveResponseTime(p)
	wantBlockResponseDownloadAvg := prt.wantBlockResponseDownloadAvg(p)

	fmt.Println("(GetValue) Peer: ", p, ", lastWantHaveResponseTime: ", lastWantHaveResponseTime, ", wantBlockResponseDownloadAvg: ", wantBlockResponseDownloadAvg)
	
	baseInversePeerValue := (a * lastWantHaveResponseTime + b * wantBlockResponseDownloadAvg)

	inversePeerValue := baseInversePeerValue * baseInversePeerValue * baseInversePeerValue * baseInversePeerValue * baseInversePeerValue * baseInversePeerValue * baseInversePeerValue * baseInversePeerValue * baseInversePeerValue * baseInversePeerValue * baseInversePeerValue * baseInversePeerValue * baseInversePeerValue * baseInversePeerValue * baseInversePeerValue

	peerValue := 1 / inversePeerValue

	return peerValue
}

func (prt *peerResponseTracker) lastWantHaveResponseTime(p peer.ID) float64 {
	duration := float64(prt.lastHaveResponseDuration[p]) 
	
	if duration == 0 {
		duration = 1000 // TODO: Set this value appropriately, we may set it to zero 
	}

	return duration
}

func (prt *peerResponseTracker) wantBlockResponseDownloadAvg(p peer.ID) float64 {
	duration := float64(prt.avgBlockResponseDuration[p]) 
	
	// TODO: Bu değer silinecek
	if duration == 0 {
		duration = 1000 // TODO: Set this value appropriately
	}

	return duration
}
