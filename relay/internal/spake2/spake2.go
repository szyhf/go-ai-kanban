// Package spake2 implements the SPAKE2 password-authenticated key exchange
// protocol over the Ed25519 curve. It is compatible with the frontend
// TypeScript implementation in packages/web-core/src/shared/lib/relayPake.ts.
package spake2

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/hkdf"
)

const (
	roleClient byte = 0x41 // 'A'
	roleServer byte = 0x42 // 'B'
)

var (
	spake2ClientID     = []byte("vibe-kanban-browser")
	spake2ServerID     = []byte("vibe-kanban-server")
	spake2PasswordInfo = []byte("SPAKE2 pw")

	keyConfirmationInfo = []byte("key-confirmation")
	clientProofContext  = []byte("vk-spake2-client-proof-v2")
	serverProofContext  = []byte("vk-spake2-server-proof-v2")

	// pointM and pointN are the SPAKE2 constant points decoded from hex.
	pointM, pointN *edwards25519.Point
)

func init() {
	var err error
	pointM, err = decodeHexPoint("15cfd18e385952982b6a8f8c7854963b58e34388c8e6dae891db756481a02312")
	if err != nil {
		panic(fmt.Sprintf("spake2: invalid M point: %v", err))
	}
	pointN, err = decodeHexPoint("f04f2e7eb734b2a8f8b472eaf9c3c632576ac64aea650b496a8a20ff00e583c3")
	if err != nil {
		panic(fmt.Sprintf("spake2: invalid N point: %v", err))
	}
}

// decodeHexPoint decodes a hex-encoded 32-byte compressed Ed25519 point.
func decodeHexPoint(h string) (*edwards25519.Point, error) {
	b, err := hex.DecodeString(h)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}
	p := new(edwards25519.Point)
	if _, err := p.SetBytes(b); err != nil {
		return nil, fmt.Errorf("invalid point: %w", err)
	}
	return p, nil
}

// State holds the ephemeral state for one side of the SPAKE2 exchange.
type State struct {
	scalar        *edwards25519.Scalar // random scalar (x or y)
	passwordW     *edwards25519.Scalar // password-derived scalar
	isClient      bool
	selfMsg       []byte // own 32-byte point (no role prefix)
	passwordBytes []byte // raw password bytes for transcript hashing
}

// StartClient initiates the SPAKE2 exchange from the client side.
// Returns the 33-byte client message to send to the server and internal state.
func StartClient(enrollmentCode string) (message []byte, state *State, err error) {
	return start(enrollmentCode, true, pointM)
}

// StartServer initiates the SPAKE2 exchange from the server side.
// Returns the 33-byte server message to send to the client and internal state.
func StartServer(enrollmentCode string) (message []byte, state *State, err error) {
	return start(enrollmentCode, false, pointN)
}

func start(enrollmentCode string, isClient bool, constant *edwards25519.Point) (message []byte, state *State, err error) {
	pwBytes := []byte(enrollmentCode)

	pwScalar, err := passwordToScalar(pwBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("password scalar derivation: %w", err)
	}

	randScalar, err := randomScalar()
	if err != nil {
		return nil, nil, fmt.Errorf("random scalar generation: %w", err)
	}

	// Compute message point: G*randScalar + constant*pwScalar
	gx := new(edwards25519.Point).ScalarMult(randScalar, edwards25519.NewGeneratorPoint())
	cp := new(edwards25519.Point).ScalarMult(pwScalar, constant)
	msgPoint := new(edwards25519.Point).Add(gx, cp)

	msgPointBytes := msgPoint.Bytes()

	role := roleClient
	if !isClient {
		role = roleServer
	}

	message = make([]byte, 33)
	message[0] = role
	copy(message[1:], msgPointBytes)

	selfMsgCopy := make([]byte, 32)
	copy(selfMsgCopy, msgPointBytes)

	pwCopy := make([]byte, len(pwBytes))
	copy(pwCopy, pwBytes)

	state = &State{
		scalar:        randScalar,
		passwordW:     pwScalar,
		isClient:      isClient,
		selfMsg:       selfMsgCopy,
		passwordBytes: pwCopy,
	}

	return message, state, nil
}

// Finish completes the SPAKE2 exchange using the peer's message.
// Returns the 32-byte shared key.
func Finish(state *State, peerMessage []byte) (sharedKey []byte, err error) {
	if len(peerMessage) != 33 {
		return nil, errors.New("peer message must be 33 bytes")
	}

	expectedRole := roleServer
	if !state.isClient {
		expectedRole = roleClient
	}
	if peerMessage[0] != expectedRole {
		return nil, fmt.Errorf("unexpected peer role byte 0x%02x, expected 0x%02x", peerMessage[0], expectedRole)
	}

	peerPointBytes := peerMessage[1:]
	peerPoint := new(edwards25519.Point)
	if _, err := peerPoint.SetBytes(peerPointBytes); err != nil {
		return nil, fmt.Errorf("invalid peer point: %w", err)
	}

	// K = (peerPoint - constant*pwScalar) * ownScalar
	constant := pointN
	if !state.isClient {
		constant = pointM
	}

	cp := new(edwards25519.Point).ScalarMult(state.passwordW, constant)
	kBase := new(edwards25519.Point).Subtract(peerPoint, cp)
	keyPoint := new(edwards25519.Point).ScalarMult(state.scalar, kBase)
	keyPointBytes := keyPoint.Bytes()

	// Build transcript exactly as the frontend hashAb function:
	// transcript = SHA256(pw) || SHA256(idA) || SHA256(idB) || clientMsg || serverMsg || K
	var clientMsg, serverMsg []byte
	if state.isClient {
		clientMsg = state.selfMsg
		serverMsg = peerPointBytes
	} else {
		clientMsg = peerPointBytes
		serverMsg = state.selfMsg
	}

	transcript := make([]byte, 6*32)
	h0 := sha256.Sum256(state.passwordBytes)
	copy(transcript[0:32], h0[:])
	h1 := sha256.Sum256(spake2ClientID)
	copy(transcript[32:64], h1[:])
	h2 := sha256.Sum256(spake2ServerID)
	copy(transcript[64:96], h2[:])
	copy(transcript[96:128], clientMsg)
	copy(transcript[128:160], serverMsg)
	copy(transcript[160:192], keyPointBytes)

	sharedKeyHash := sha256.Sum256(transcript)
	sharedKey = make([]byte, 32)
	copy(sharedKey, sharedKeyHash[:])

	return sharedKey, nil
}

// GenerateClientProof generates the client's key confirmation proof.
// Matches frontend buildClientProofB64:
//
//	HMAC-SHA256(confirmationKey, CLIENT_PROOF_CONTEXT + enrollmentIdBytes + browserPublicKeyBytes)
func GenerateClientProof(sharedKey []byte, enrollmentID string, browserPublicKey []byte) ([]byte, error) {
	confirmationKey, err := deriveConfirmationKey(sharedKey)
	if err != nil {
		return nil, err
	}
	enrollmentBytes := uuidToBytes(enrollmentID)
	payload := concatBytes(clientProofContext, enrollmentBytes, browserPublicKey)
	return hmacSHA256(confirmationKey, payload), nil
}

// GenerateServerProof generates the server's key confirmation proof.
// Matches frontend verifyServerProof:
//
//	HMAC-SHA256(confirmationKey, SERVER_PROOF_CONTEXT + enrollmentIdBytes + browserPublicKeyBytes + serverPublicKeyBytes)
func GenerateServerProof(sharedKey []byte, enrollmentID string, browserPublicKey, serverPublicKey []byte) ([]byte, error) {
	confirmationKey, err := deriveConfirmationKey(sharedKey)
	if err != nil {
		return nil, err
	}
	enrollmentBytes := uuidToBytes(enrollmentID)
	payload := concatBytes(serverProofContext, enrollmentBytes, browserPublicKey, serverPublicKey)
	return hmacSHA256(confirmationKey, payload), nil
}

// VerifyClientProof checks the client's proof.
func VerifyClientProof(sharedKey []byte, enrollmentID string, browserPublicKey, proof []byte) bool {
	expected, err := GenerateClientProof(sharedKey, enrollmentID, browserPublicKey)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, proof)
}

// VerifyServerProof checks the server's proof.
func VerifyServerProof(sharedKey []byte, enrollmentID string, browserPublicKey, serverPublicKey, proof []byte) bool {
	expected, err := GenerateServerProof(sharedKey, enrollmentID, browserPublicKey, serverPublicKey)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, proof)
}

// uuidToBytes converts a UUID string to 16 bytes (stripping dashes).
func uuidToBytes(id string) []byte {
	h := strings.ReplaceAll(id, "-", "")
	b, _ := hex.DecodeString(h)
	return b
}

// concatBytes concatenates multiple byte slices.
func concatBytes(chunks ...[]byte) []byte {
	total := 0
	for _, c := range chunks {
		total += len(c)
	}
	out := make([]byte, total)
	offset := 0
	for _, c := range chunks {
		copy(out[offset:], c)
		offset += len(c)
	}
	return out
}

// ParsePeerMessage validates and extracts the 32-byte point from a peer message.
// Returns the role byte (0x41 or 0x42) and the point bytes.
func ParsePeerMessage(msg []byte) (role byte, point []byte, err error) {
	if len(msg) != 33 {
		return 0, nil, errors.New("peer message must be 33 bytes")
	}
	role = msg[0]
	if role != roleClient && role != roleServer {
		return 0, nil, fmt.Errorf("invalid role byte 0x%02x", role)
	}
	point = make([]byte, 32)
	copy(point, msg[1:])
	return role, point, nil
}

// passwordToScalar converts a password to a scalar using HKDF.
// Matches the frontend hashToSpake2Scalar:
//   - HKDF-SHA256(key=pw, salt=empty, info="SPAKE2 pw", len=48) -> 48-byte okm
//   - Reverse okm into a 64-byte LE buffer (high 16 bytes zero)
//   - Reduce mod l using SetUniformBytes
func passwordToScalar(pw []byte) (*edwards25519.Scalar, error) {
	okm := make([]byte, 48)
	hd := hkdf.New(sha256.New, pw, nil, spake2PasswordInfo)
	if _, err := hd.Read(okm); err != nil {
		return nil, fmt.Errorf("HKDF expand: %w", err)
	}

	// The frontend reverses the 48 okm bytes into a 64-byte buffer where
	// reducible[0..47] = reversed(okm) and reducible[48..63] = 0.
	// This is equivalent to interpreting okm as a big-endian number stored
	// in the low 48 bytes of a 64-byte little-endian integer.
	// Since SetUniformBytes takes 64 bytes LE and reduces mod l, we can
	// build the 64-byte LE representation directly.
	le64 := make([]byte, 64)
	for i := 0; i < 48; i++ {
		le64[47-i] = okm[i]
	}
	// le64[48..63] are already zero

	s := new(edwards25519.Scalar)
	if _, err := s.SetUniformBytes(le64); err != nil {
		return nil, fmt.Errorf("set scalar from password: %w", err)
	}
	return s, nil
}

// randomScalar generates a uniformly random scalar in [0, l).
// Matches the frontend randomScalar: 64 random bytes -> LE integer mod l.
func randomScalar() (*edwards25519.Scalar, error) {
	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("read random bytes: %w", err)
	}
	s := new(edwards25519.Scalar)
	if _, err := s.SetUniformBytes(buf); err != nil {
		return nil, fmt.Errorf("set random scalar: %w", err)
	}
	return s, nil
}

// deriveConfirmationKey derives a 32-byte confirmation key from the shared key.
func deriveConfirmationKey(sharedKey []byte) ([]byte, error) {
	okm := make([]byte, 32)
	hd := hkdf.New(sha256.New, sharedKey, nil, keyConfirmationInfo)
	if _, err := hd.Read(okm); err != nil {
		return nil, fmt.Errorf("HKDF expand for confirmation key: %w", err)
	}
	return okm, nil
}

// hmacSHA256 computes HMAC-SHA256.
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
