package session

import (
	"math/rand"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

// peerResponseTracker keeps track of how many times each peer was the first
// to send us a block for a given CID (used to rank peers)
type peerResponseTracker struct {
	firstResponder map[peer.ID]int
}

func newPeerResponseTracker() *peerResponseTracker {
	return &peerResponseTracker{
		firstResponder: make(map[peer.ID]int),
	}
}

// receivedBlockFrom is called when a block is received from a peer
// (only called first time block is received)
func (prt *peerResponseTracker) receivedBlockFrom(from peer.ID) {
	prt.firstResponder[from]++
}

// choose picks a peer from the list of candidate peers, favouring those peers
// that were first to send us previous blocks
func (prt *peerResponseTracker) choose(peers []peer.ID) peer.ID {
	if len(peers) == 0 {
		return ""
	}

	// find the peer that responded first at most
	mostResponderPeer := peer.ID("")
	maxRespondCount := 0
	for _, p := range peers {
		var respondCount = prt.getFirstRespondCountForPeer(p)
		if respondCount >= maxRespondCount {
			maxRespondCount = respondCount
			mostResponderPeer = p
		}
	}

	return mostResponderPeer
}

// getFirstRespondCountForPeer returns the number of times the peer was first to send us a
// block plus one (in order to never get a zero chance).
func (prt *peerResponseTracker) getFirstRespondCountForPeer(p peer.ID) int {
	// Make sure there is always at least a small chance a new peer
	// will be chosen
	return prt.firstResponder[p] + 1
}
