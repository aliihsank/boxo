package session

import (
	"fmt"
	"fmt"
	"math/rand"
	"container/list"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// peerResponseTracker keeps track of how many times each peer was the first
// to send us a block for a given CID (used to rank peers)
type peerResponseTracker struct {
	firstResponder map[peer.ID]int
	respondQueue *list.List
	respondCountTreshold int
}

func newPeerResponseTracker() *peerResponseTracker {
	return &peerResponseTracker{
		firstResponder: make(map[peer.ID]int),
		respondQueue: list.New(),
		respondCountTreshold: 1000,
	}
}

// receivedBlockFrom is called when a block is received from a peer
// (only called first time block is received)
func (prt *peerResponseTracker) receivedBlockFrom(from peer.ID) {
	prt.firstResponder[from]++

	prt.respondQueue.PushBack(from)
	
	if prt.respondQueue.Len() > prt.respondCountTreshold {
		front := prt.respondQueue.Front()
		
		prt.firstResponder[front.Value.(peer.ID)]--

		prt.respondQueue.Remove(front)
	}
	
	fmt.Println("Received Block response from: ", from)
}

// choose picks a peer from the list of candidate peers, favouring those peers
// that were first to send us previous blocks
func (prt *peerResponseTracker) choose(peers []peer.ID) peer.ID {
	if len(peers) == 0 {
		return ""
	}

	rnd := rand.Float64()

	// Find the total received blocks for all candidate peers
	total := 0
	for _, p := range peers {
		total += prt.getFirstRespondCountForPeer(p)
	}

	// Choose one of the peers with a chance proportional to the number
	// of blocks received from that peer
	counted := 0.0
	for _, p := range peers {
		counted += float64(prt.getFirstRespondCountForPeer(p)) / float64(total)
		if counted > rnd {
			return p
		}
	}

	// We shouldn't get here unless there is some weirdness with floating point
	// math that doesn't quite cover the whole range of peers in the for loop
	// so just choose the last peer.
	index := len(peers) - 1
	return peers[index]
}

// getFirstRespondCountForPeer returns the number of times the peer was first to send us a
// block plus one (in order to never get a zero chance).
func (prt *peerResponseTracker) getFirstRespondCountForPeer(p peer.ID) int {
	// Make sure there is always at least a small chance a new peer
	// will be chosen
	return prt.firstResponder[p] + 1
}
