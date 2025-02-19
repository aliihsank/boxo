package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/gorilla/mux"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/routing/http/internal/drjson"
	"github.com/ipfs/boxo/routing/http/types"
	"github.com/ipfs/boxo/routing/http/types/iter"
	jsontypes "github.com/ipfs/boxo/routing/http/types/json"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	logging "github.com/ipfs/go-log/v2"
)

const (
	mediaTypeJSON       = "application/json"
	mediaTypeNDJSON     = "application/x-ndjson"
	mediaTypeWildcard   = "*/*"
	mediaTypeIPNSRecord = "application/vnd.ipfs.ipns-record"

	DefaultRecordsLimit          = 20
	DefaultStreamingRecordsLimit = 0
)

var logger = logging.Logger("routing/http/server")

const (
	providePath       = "/routing/v1/providers/"
	findProvidersPath = "/routing/v1/providers/{cid}"
	findPeersPath     = "/routing/v1/peers/{peer-id}"
	GetIPNSPath       = "/routing/v1/ipns/{cid}"
)

type FindProvidersAsyncResponse struct {
	ProviderResponse types.Record
	Error            error
}

type ContentRouter interface {
	// FindProviders searches for peers who are able to provide the given [cid.Cid].
	// Limit indicates the maximum amount of results to return; 0 means unbounded.
	FindProviders(ctx context.Context, cid cid.Cid, limit int) (iter.ResultIter[types.Record], error)

	// Deprecated: protocol-agnostic provide is being worked on in [IPIP-378]:
	//
	// [IPIP-378]: https://github.com/ipfs/specs/pull/378
	ProvideBitswap(ctx context.Context, req *BitswapWriteProvideRequest) (time.Duration, error)

	// FindPeers searches for peers who have the provided [peer.ID].
	// Limit indicates the maximum amount of results to return; 0 means unbounded.
	FindPeers(ctx context.Context, pid peer.ID, limit int) (iter.ResultIter[*types.PeerRecord], error)

	// GetIPNS searches for an [ipns.Record] for the given [ipns.Name].
	GetIPNS(ctx context.Context, name ipns.Name) (*ipns.Record, error)

	// PutIPNS stores the provided [ipns.Record] for the given [ipns.Name].
	// It is guaranteed that the record matches the provided name.
	PutIPNS(ctx context.Context, name ipns.Name, record *ipns.Record) error
}

// Deprecated: protocol-agnostic provide is being worked on in [IPIP-378]:
//
// [IPIP-378]: https://github.com/ipfs/specs/pull/378
type BitswapWriteProvideRequest struct {
	Keys        []cid.Cid
	Timestamp   time.Time
	AdvisoryTTL time.Duration
	ID          peer.ID
	Addrs       []multiaddr.Multiaddr
}

// Deprecated: protocol-agnostic provide is being worked on in [IPIP-378]:
//
// [IPIP-378]: https://github.com/ipfs/specs/pull/378
type WriteProvideRequest struct {
	Protocol string
	Schema   string
	Bytes    []byte
}

type Option func(s *server)

// WithStreamingResultsDisabled disables ndjson responses, so that the server only supports JSON responses.
func WithStreamingResultsDisabled() Option {
	return func(s *server) {
		s.disableNDJSON = true
	}
}

// WithRecordsLimit sets a limit that will be passed to [ContentRouter.FindProviders]
// and [ContentRouter.FindPeers] for non-streaming requests (application/json).
// Default is [DefaultRecordsLimit].
func WithRecordsLimit(limit int) Option {
	return func(s *server) {
		s.recordsLimit = limit
	}
}

// WithStreamingRecordsLimit sets a limit that will be passed to [ContentRouter.FindProviders]
// and [ContentRouter.FindPeers] for streaming requests (application/x-ndjson).
// Default is [DefaultStreamingRecordsLimit].
func WithStreamingRecordsLimit(limit int) Option {
	return func(s *server) {
		s.streamingRecordsLimit = limit
	}
}

func Handler(svc ContentRouter, opts ...Option) http.Handler {
	server := &server{
		svc:                   svc,
		recordsLimit:          DefaultRecordsLimit,
		streamingRecordsLimit: DefaultStreamingRecordsLimit,
	}

	for _, opt := range opts {
		opt(server)
	}

	r := mux.NewRouter()
	r.HandleFunc(findProvidersPath, server.findProviders).Methods(http.MethodGet)
	r.HandleFunc(providePath, server.provide).Methods(http.MethodPut)
	r.HandleFunc(findPeersPath, server.findPeers).Methods(http.MethodGet)
	r.HandleFunc(GetIPNSPath, server.GetIPNS).Methods(http.MethodGet)
	r.HandleFunc(GetIPNSPath, server.PutIPNS).Methods(http.MethodPut)
	return r
}

type server struct {
	svc                   ContentRouter
	disableNDJSON         bool
	recordsLimit          int
	streamingRecordsLimit int
}

func (s *server) detectResponseType(r *http.Request) (string, error) {
	var (
		supportsNDJSON bool
		supportsJSON   bool

		acceptHeaders = r.Header.Values("Accept")
	)

	if len(acceptHeaders) == 0 {
		return mediaTypeJSON, nil
	}

	for _, acceptHeader := range acceptHeaders {
		for _, accept := range strings.Split(acceptHeader, ",") {
			mediaType, _, err := mime.ParseMediaType(accept)
			if err != nil {
				return "", fmt.Errorf("unable to parse Accept header: %w", err)
			}

			switch mediaType {
			case mediaTypeJSON, mediaTypeWildcard:
				supportsJSON = true
			case mediaTypeNDJSON:
				supportsNDJSON = true
			}
		}
	}

	if supportsNDJSON && !s.disableNDJSON {
		return mediaTypeNDJSON, nil
	} else if supportsJSON {
		return mediaTypeJSON, nil
	} else {
		return "", errors.New("no supported content types")
	}
}

func (s *server) findProviders(w http.ResponseWriter, httpReq *http.Request) {
	vars := mux.Vars(httpReq)
	cidStr := vars["cid"]
	cid, err := cid.Decode(cidStr)
	if err != nil {
		writeErr(w, "FindProviders", http.StatusBadRequest, fmt.Errorf("unable to parse CID: %w", err))
		return
	}

	mediaType, err := s.detectResponseType(httpReq)
	if err != nil {
		writeErr(w, "FindProviders", http.StatusBadRequest, err)
		return
	}

	var (
		handlerFunc  func(w http.ResponseWriter, provIter iter.ResultIter[types.Record])
		recordsLimit int
	)

	if mediaType == mediaTypeNDJSON {
		handlerFunc = s.findProvidersNDJSON
		recordsLimit = s.streamingRecordsLimit
	} else {
		handlerFunc = s.findProvidersJSON
		recordsLimit = s.recordsLimit
	}

	provIter, err := s.svc.FindProviders(httpReq.Context(), cid, recordsLimit)
	if err != nil {
		writeErr(w, "FindProviders", http.StatusInternalServerError, fmt.Errorf("delegate error: %w", err))
		return
	}

	handlerFunc(w, provIter)
}

func (s *server) findProvidersJSON(w http.ResponseWriter, provIter iter.ResultIter[types.Record]) {
	defer provIter.Close()

	providers, err := iter.ReadAllResults(provIter)
	if err != nil {
		writeErr(w, "FindProviders", http.StatusInternalServerError, fmt.Errorf("delegate error: %w", err))
		return
	}

	writeJSONResult(w, "FindProviders", jsontypes.ProvidersResponse{
		Providers: providers,
	})
}

func (s *server) findProvidersNDJSON(w http.ResponseWriter, provIter iter.ResultIter[types.Record]) {
	writeResultsIterNDJSON(w, provIter)
}

func (s *server) findPeers(w http.ResponseWriter, r *http.Request) {
	pidStr := mux.Vars(r)["peer-id"]

	// pidStr must be in CIDv1 format. Therefore, use [cid.Decode]. We can't use
	// [peer.Decode] because that would allow other formats to pass through.
	cid, err := cid.Decode(pidStr)
	if err != nil {
		if pid, err := peer.Decode(pidStr); err == nil {
			writeErr(w, "FindPeers", http.StatusBadRequest, fmt.Errorf("the value is a peer ID, try using its CID representation: %s", peer.ToCid(pid).String()))
		} else {
			writeErr(w, "FindPeers", http.StatusBadRequest, fmt.Errorf("unable to parse peer ID: %w", err))
		}
		return
	}

	pid, err := peer.FromCid(cid)
	if err != nil {
		writeErr(w, "FindPeers", http.StatusBadRequest, fmt.Errorf("unable to parse peer ID: %w", err))
		return
	}

	mediaType, err := s.detectResponseType(r)
	if err != nil {
		writeErr(w, "FindPeers", http.StatusBadRequest, err)
		return
	}

	var (
		handlerFunc  func(w http.ResponseWriter, provIter iter.ResultIter[*types.PeerRecord])
		recordsLimit int
	)

	if mediaType == mediaTypeNDJSON {
		handlerFunc = s.findPeersNDJSON
		recordsLimit = s.streamingRecordsLimit
	} else {
		handlerFunc = s.findPeersJSON
		recordsLimit = s.recordsLimit
	}

	provIter, err := s.svc.FindPeers(r.Context(), pid, recordsLimit)
	if err != nil {
		writeErr(w, "FindPeers", http.StatusInternalServerError, fmt.Errorf("delegate error: %w", err))
		return
	}

	handlerFunc(w, provIter)
}

func (s *server) provide(w http.ResponseWriter, httpReq *http.Request) {
	//lint:ignore SA1019 // ignore staticcheck
	req := jsontypes.WriteProvidersRequest{}
	err := json.NewDecoder(httpReq.Body).Decode(&req)
	_ = httpReq.Body.Close()
	if err != nil {
		writeErr(w, "Provide", http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}

	//lint:ignore SA1019 // ignore staticcheck
	resp := jsontypes.WriteProvidersResponse{}

	for i, prov := range req.Providers {
		switch v := prov.(type) {
		//lint:ignore SA1019 // ignore staticcheck
		case *types.WriteBitswapRecord:
			err := v.Verify()
			if err != nil {
				logErr("Provide", "signature verification failed", err)
				writeErr(w, "Provide", http.StatusForbidden, errors.New("signature verification failed"))
				return
			}

			keys := make([]cid.Cid, len(v.Payload.Keys))
			for i, k := range v.Payload.Keys {
				keys[i] = k.Cid
			}
			addrs := make([]multiaddr.Multiaddr, len(v.Payload.Addrs))
			for i, a := range v.Payload.Addrs {
				addrs[i] = a.Multiaddr
			}
			advisoryTTL, err := s.svc.ProvideBitswap(httpReq.Context(), &BitswapWriteProvideRequest{
				Keys:        keys,
				Timestamp:   v.Payload.Timestamp.Time,
				AdvisoryTTL: v.Payload.AdvisoryTTL.Duration,
				ID:          *v.Payload.ID,
				Addrs:       addrs,
			})
			if err != nil {
				writeErr(w, "Provide", http.StatusInternalServerError, fmt.Errorf("delegate error: %w", err))
				return
			}
			resp.ProvideResults = append(resp.ProvideResults,
				//lint:ignore SA1019 // ignore staticcheck
				&types.WriteBitswapRecordResponse{
					Protocol:    v.Protocol,
					Schema:      v.Schema,
					AdvisoryTTL: &types.Duration{Duration: advisoryTTL},
				},
			)
		default:
			writeErr(w, "Provide", http.StatusBadRequest, fmt.Errorf("provider record %d is not bitswap", i))
			return
		}
	}
	writeJSONResult(w, "Provide", resp)
}

func (s *server) findPeersJSON(w http.ResponseWriter, peersIter iter.ResultIter[*types.PeerRecord]) {
	defer peersIter.Close()

	peers, err := iter.ReadAllResults(peersIter)
	if err != nil {
		writeErr(w, "FindPeers", http.StatusInternalServerError, fmt.Errorf("delegate error: %w", err))
		return
	}

	writeJSONResult(w, "FindPeers", jsontypes.PeersResponse{
		Peers: peers,
	})
}

func (s *server) findPeersNDJSON(w http.ResponseWriter, peersIter iter.ResultIter[*types.PeerRecord]) {
	writeResultsIterNDJSON(w, peersIter)
}

func (s *server) GetIPNS(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Accept"), mediaTypeIPNSRecord) {
		writeErr(w, "GetIPNS", http.StatusNotAcceptable, errors.New("content type in 'Accept' header is missing or not supported"))
		return
	}

	vars := mux.Vars(r)
	cidStr := vars["cid"]
	cid, err := cid.Decode(cidStr)
	if err != nil {
		writeErr(w, "GetIPNS", http.StatusBadRequest, fmt.Errorf("unable to parse CID: %w", err))
		return
	}

	name, err := ipns.NameFromCid(cid)
	if err != nil {
		writeErr(w, "GetIPNS", http.StatusBadRequest, fmt.Errorf("peer ID CID is not valid: %w", err))
		return
	}

	record, err := s.svc.GetIPNS(r.Context(), name)
	if err != nil {
		writeErr(w, "GetIPNS", http.StatusInternalServerError, fmt.Errorf("delegate error: %w", err))
		return
	}

	rawRecord, err := ipns.MarshalRecord(record)
	if err != nil {
		writeErr(w, "GetIPNS", http.StatusInternalServerError, err)
		return
	}

	if ttl, err := record.TTL(); err == nil {
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", int(ttl.Seconds())))
	} else {
		w.Header().Set("Cache-Control", "max-age=60")
	}

	recordEtag := strconv.FormatUint(xxhash.Sum64(rawRecord), 32)
	w.Header().Set("Etag", recordEtag)
	w.Header().Set("Content-Type", mediaTypeIPNSRecord)
	w.Write(rawRecord)
}

func (s *server) PutIPNS(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Content-Type"), mediaTypeIPNSRecord) {
		writeErr(w, "PutIPNS", http.StatusNotAcceptable, errors.New("content type in 'Content-Type' header is missing or not supported"))
		return
	}

	vars := mux.Vars(r)
	cidStr := vars["cid"]
	cid, err := cid.Decode(cidStr)
	if err != nil {
		writeErr(w, "PutIPNS", http.StatusBadRequest, fmt.Errorf("unable to parse CID: %w", err))
		return
	}

	name, err := ipns.NameFromCid(cid)
	if err != nil {
		writeErr(w, "PutIPNS", http.StatusBadRequest, fmt.Errorf("peer ID CID is not valid: %w", err))
		return
	}

	// Limit the reader to the maximum record size.
	rawRecord, err := io.ReadAll(io.LimitReader(r.Body, int64(ipns.MaxRecordSize)))
	if err != nil {
		writeErr(w, "PutIPNS", http.StatusBadRequest, fmt.Errorf("provided record is too long: %w", err))
		return
	}

	record, err := ipns.UnmarshalRecord(rawRecord)
	if err != nil {
		writeErr(w, "PutIPNS", http.StatusBadRequest, fmt.Errorf("provided record is invalid: %w", err))
		return
	}

	err = ipns.ValidateWithName(record, name)
	if err != nil {
		writeErr(w, "PutIPNS", http.StatusBadRequest, fmt.Errorf("provided record is invalid: %w", err))
		return
	}

	err = s.svc.PutIPNS(r.Context(), name, record)
	if err != nil {
		writeErr(w, "PutIPNS", http.StatusInternalServerError, fmt.Errorf("delegate error: %w", err))
		return
	}

	w.WriteHeader(http.StatusOK)
}

func writeJSONResult(w http.ResponseWriter, method string, val any) {
	w.Header().Add("Content-Type", mediaTypeJSON)

	// keep the marshaling separate from the writing, so we can distinguish bugs (which surface as 500)
	// from transient network issues (which surface as transport errors)
	b, err := drjson.MarshalJSONBytes(val)
	if err != nil {
		writeErr(w, method, http.StatusInternalServerError, fmt.Errorf("marshaling response: %w", err))
		return
	}

	_, err = io.Copy(w, bytes.NewBuffer(b))
	if err != nil {
		logErr("Provide", "writing response body", err)
	}
}

func writeErr(w http.ResponseWriter, method string, statusCode int, cause error) {
	w.WriteHeader(statusCode)
	causeStr := cause.Error()
	if len(causeStr) > 1024 {
		causeStr = causeStr[:1024]
	}
	_, err := w.Write([]byte(causeStr))
	if err != nil {
		logErr(method, "error writing error cause", err)
		return
	}
}

func logErr(method, msg string, err error) {
	logger.Infow(msg, "Method", method, "Error", err)
}

func writeResultsIterNDJSON[T any](w http.ResponseWriter, resultIter iter.ResultIter[T]) {
	defer resultIter.Close()

	w.Header().Set("Content-Type", mediaTypeNDJSON)
	w.WriteHeader(http.StatusOK)

	for resultIter.Next() {
		res := resultIter.Val()
		if res.Err != nil {
			logger.Errorw("ndjson iterator error", "Error", res.Err)
			return
		}
		// don't use an encoder because we can't easily differentiate writer errors from encoding errors
		b, err := drjson.MarshalJSONBytes(res.Val)
		if err != nil {
			logger.Errorw("ndjson marshal error", "Error", err)
			return
		}

		_, err = w.Write(b)
		if err != nil {
			logger.Warn("ndjson write error", "Error", err)
			return
		}

		_, err = w.Write([]byte{'\n'})
		if err != nil {
			logger.Warn("ndjson write error", "Error", err)
			return
		}

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}
