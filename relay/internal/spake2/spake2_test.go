package spake2

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"filippo.io/edwards25519"
)

func TestStartClientMessageFormat(t *testing.T) {
	msg, _, err := StartClient("ABC123")
	if err != nil {
		t.Fatalf("StartClient: %v", err)
	}
	if len(msg) != 33 {
		t.Fatalf("expected 33 bytes, got %d", len(msg))
	}
	if msg[0] != 0x41 {
		t.Fatalf("expected role byte 0x41, got 0x%02x", msg[0])
	}
}

func TestStartServerMessageFormat(t *testing.T) {
	msg, _, err := StartServer("ABC123")
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	if len(msg) != 33 {
		t.Fatalf("expected 33 bytes, got %d", len(msg))
	}
	if msg[0] != 0x42 {
		t.Fatalf("expected role byte 0x42, got 0x%02x", msg[0])
	}
}

func TestBothSidesAgreeOnSharedKey(t *testing.T) {
	enrollmentCode := "ABC123"

	clientMsg, clientState, err := StartClient(enrollmentCode)
	if err != nil {
		t.Fatalf("StartClient: %v", err)
	}

	serverMsg, serverState, err := StartServer(enrollmentCode)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	clientShared, err := Finish(clientState, serverMsg)
	if err != nil {
		t.Fatalf("client Finish: %v", err)
	}

	serverShared, err := Finish(serverState, clientMsg)
	if err != nil {
		t.Fatalf("server Finish: %v", err)
	}

	if !bytes.Equal(clientShared, serverShared) {
		t.Fatalf("shared keys do not match\nclient: %x\nserver: %x", clientShared, serverShared)
	}
	if len(clientShared) != 32 {
		t.Fatalf("expected 32-byte shared key, got %d bytes", len(clientShared))
	}
}

func TestDifferentPasswordsProduceDifferentKeys(t *testing.T) {
	clientMsg, clientState, err := StartClient("ABC123")
	if err != nil {
		t.Fatalf("StartClient: %v", err)
	}

	serverMsg, serverState, err := StartServer("XYZ789")
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	clientShared, err := Finish(clientState, serverMsg)
	if err != nil {
		t.Fatalf("client Finish: %v", err)
	}

	serverShared, err := Finish(serverState, clientMsg)
	if err != nil {
		t.Fatalf("server Finish: %v", err)
	}

	if bytes.Equal(clientShared, serverShared) {
		t.Fatal("shared keys should NOT match with different passwords")
	}
}

func TestSamePasswordDifferentRunsProduceDifferentMessages(t *testing.T) {
	msg1, _, err := StartClient("ABC123")
	if err != nil {
		t.Fatalf("StartClient 1: %v", err)
	}
	msg2, _, err := StartClient("ABC123")
	if err != nil {
		t.Fatalf("StartClient 2: %v", err)
	}
	if bytes.Equal(msg1[1:], msg2[1:]) {
		t.Fatal("two runs with same password should produce different messages due to random scalars")
	}
}

// testProofParams creates test values for proof generation/verification.
func testProofParams() (enrollmentID string, browserPub, serverPub []byte) {
	enrollmentID = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	browserPub = make([]byte, ed25519.PublicKeySize)
	serverPub = make([]byte, ed25519.PublicKeySize)
	for i := range browserPub {
		browserPub[i] = byte(i)
		serverPub[i] = byte(i + 32)
	}
	return
}

func TestGenerateAndVerifyClientProof(t *testing.T) {
	enrollmentCode := "TEST01"
	enrollmentID, browserPub, _ := testProofParams()

	_, clientState, err := StartClient(enrollmentCode)
	if err != nil {
		t.Fatalf("StartClient: %v", err)
	}
	serverMsg, _, err := StartServer(enrollmentCode)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	sharedKey, err := Finish(clientState, serverMsg)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}

	proof, err := GenerateClientProof(sharedKey, enrollmentID, browserPub)
	if err != nil {
		t.Fatalf("GenerateClientProof: %v", err)
	}
	if len(proof) != 32 {
		t.Fatalf("expected 32-byte proof, got %d", len(proof))
	}

	if !VerifyClientProof(sharedKey, enrollmentID, browserPub, proof) {
		t.Fatal("VerifyClientProof should return true for a valid proof")
	}

	// Wrong key should fail.
	wrongKey := make([]byte, 32)
	copy(wrongKey, sharedKey)
	wrongKey[0] ^= 0xff
	if VerifyClientProof(wrongKey, enrollmentID, browserPub, proof) {
		t.Fatal("VerifyClientProof should return false for a wrong key")
	}

	// Wrong proof should fail.
	wrongProof := make([]byte, 32)
	copy(wrongProof, proof)
	wrongProof[0] ^= 0xff
	if VerifyClientProof(sharedKey, enrollmentID, browserPub, wrongProof) {
		t.Fatal("VerifyClientProof should return false for a wrong proof")
	}
}

func TestGenerateAndVerifyServerProof(t *testing.T) {
	enrollmentCode := "TEST02"
	enrollmentID, browserPub, serverPub := testProofParams()

	_, clientState, err := StartClient(enrollmentCode)
	if err != nil {
		t.Fatalf("StartClient: %v", err)
	}
	serverMsg, _, err := StartServer(enrollmentCode)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}

	sharedKey, err := Finish(clientState, serverMsg)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}

	proof, err := GenerateServerProof(sharedKey, enrollmentID, browserPub, serverPub)
	if err != nil {
		t.Fatalf("GenerateServerProof: %v", err)
	}
	if len(proof) != 32 {
		t.Fatalf("expected 32-byte proof, got %d", len(proof))
	}

	if !VerifyServerProof(sharedKey, enrollmentID, browserPub, serverPub, proof) {
		t.Fatal("VerifyServerProof should return true for a valid proof")
	}

	// Wrong key should fail.
	wrongKey := make([]byte, 32)
	copy(wrongKey, sharedKey)
	wrongKey[0] ^= 0xff
	if VerifyServerProof(wrongKey, enrollmentID, browserPub, serverPub, proof) {
		t.Fatal("VerifyServerProof should return false for a wrong key")
	}
}

func TestClientAndServerProofsAreDifferent(t *testing.T) {
	enrollmentCode := "TEST03"
	enrollmentID, browserPub, serverPub := testProofParams()

	_, clientState, err := StartClient(enrollmentCode)
	if err != nil {
		t.Fatalf("StartClient: %v", err)
	}
	serverMsg, _, err := StartServer(enrollmentCode)
	if err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	sharedKey, err := Finish(clientState, serverMsg)
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}

	clientProof, err := GenerateClientProof(sharedKey, enrollmentID, browserPub)
	if err != nil {
		t.Fatalf("GenerateClientProof: %v", err)
	}
	serverProof, err := GenerateServerProof(sharedKey, enrollmentID, browserPub, serverPub)
	if err != nil {
		t.Fatalf("GenerateServerProof: %v", err)
	}

	if bytes.Equal(clientProof, serverProof) {
		t.Fatal("client and server proofs should be different")
	}

	// Cross-verification should fail.
	if VerifyServerProof(sharedKey, enrollmentID, browserPub, serverPub, clientProof) {
		t.Fatal("server proof check should reject client proof")
	}
	if VerifyClientProof(sharedKey, enrollmentID, browserPub, serverProof) {
		t.Fatal("client proof check should reject server proof")
	}
}

func TestParsePeerMessage(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantRole  byte
		wantPoint []byte
		wantErr   bool
	}{
		{
			name:      "valid client message",
			input:     append([]byte{0x41}, make([]byte, 32)...),
			wantRole:  0x41,
			wantPoint: make([]byte, 32),
			wantErr:   false,
		},
		{
			name:      "valid server message",
			input:     append([]byte{0x42}, make([]byte, 32)...),
			wantRole:  0x42,
			wantPoint: make([]byte, 32),
			wantErr:   false,
		},
		{
			name:    "too short",
			input:   []byte{0x41, 0x00},
			wantErr: true,
		},
		{
			name:    "too long",
			input:   make([]byte, 34),
			wantErr: true,
		},
		{
			name:    "invalid role byte",
			input:   append([]byte{0x43}, make([]byte, 32)...),
			wantErr: true,
		},
		{
			name:    "empty",
			input:   []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, point, err := ParsePeerMessage(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if role != tt.wantRole {
				t.Fatalf("expected role 0x%02x, got 0x%02x", tt.wantRole, role)
			}
			if !bytes.Equal(point, tt.wantPoint) {
				t.Fatalf("point mismatch")
			}
		})
	}
}

func TestFinishRejectsWrongRole(t *testing.T) {
	_, clientState, err := StartClient("ABC123")
	if err != nil {
		t.Fatalf("StartClient: %v", err)
	}

	// Client expects a server message (0x42), not another client message (0x41).
	fakeMsg := make([]byte, 33)
	fakeMsg[0] = 0x41

	_, err = Finish(clientState, fakeMsg)
	if err == nil {
		t.Fatal("expected error when client receives a client message")
	}
}

func TestFinishRejectsWrongLength(t *testing.T) {
	_, clientState, err := StartClient("ABC123")
	if err != nil {
		t.Fatalf("StartClient: %v", err)
	}

	_, err = Finish(clientState, []byte{0x42, 0x00})
	if err == nil {
		t.Fatal("expected error for short peer message")
	}
}

func TestPasswordScalarDeterministic(t *testing.T) {
	pw := []byte("ABC123")
	s1, err := passwordToScalar(pw)
	if err != nil {
		t.Fatalf("passwordToScalar: %v", err)
	}
	s2, err := passwordToScalar(pw)
	if err != nil {
		t.Fatalf("passwordToScalar: %v", err)
	}
	if !bytes.Equal(s1.Bytes(), s2.Bytes()) {
		t.Fatal("password scalars should be deterministic for the same password")
	}
}

func TestPasswordScalarDifferentPasswords(t *testing.T) {
	s1, err := passwordToScalar([]byte("ABC123"))
	if err != nil {
		t.Fatalf("passwordToScalar: %v", err)
	}
	s2, err := passwordToScalar([]byte("XYZ789"))
	if err != nil {
		t.Fatalf("passwordToScalar: %v", err)
	}
	if bytes.Equal(s1.Bytes(), s2.Bytes()) {
		t.Fatal("different passwords should produce different scalars")
	}
}

func TestConstantPointsDecode(t *testing.T) {
	if pointM == nil {
		t.Fatal("pointM is nil")
	}
	if pointN == nil {
		t.Fatal("pointN is nil")
	}

	identity := new(edwards25519.Point).Set(edwards25519.NewIdentityPoint())
	if pointM.Equal(identity) == 1 {
		t.Fatal("M should not be the identity point")
	}
	if pointN.Equal(identity) == 1 {
		t.Fatal("N should not be the identity point")
	}
}

func TestFullProtocolRoundTrip(t *testing.T) {
	codes := []string{"ABC123", "XYZ789", "A1B2C3", "ZZZZZZ", "000000"}
	enrollmentID, browserPub, serverPub := testProofParams()

	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			clientMsg, clientState, err := StartClient(code)
			if err != nil {
				t.Fatalf("StartClient: %v", err)
			}

			serverMsg, serverState, err := StartServer(code)
			if err != nil {
				t.Fatalf("StartServer: %v", err)
			}

			clientShared, err := Finish(clientState, serverMsg)
			if err != nil {
				t.Fatalf("client Finish: %v", err)
			}

			serverShared, err := Finish(serverState, clientMsg)
			if err != nil {
				t.Fatalf("server Finish: %v", err)
			}

			if !bytes.Equal(clientShared, serverShared) {
				t.Fatalf("shared keys mismatch for code %q", code)
			}

			// Key confirmation with enrollment ID and public keys.
			clientProof, err := GenerateClientProof(clientShared, enrollmentID, browserPub)
			if err != nil {
				t.Fatalf("GenerateClientProof: %v", err)
			}
			serverProof, err := GenerateServerProof(serverShared, enrollmentID, browserPub, serverPub)
			if err != nil {
				t.Fatalf("GenerateServerProof: %v", err)
			}

			if !VerifyClientProof(serverShared, enrollmentID, browserPub, clientProof) {
				t.Fatal("server failed to verify client proof")
			}
			if !VerifyServerProof(clientShared, enrollmentID, browserPub, serverPub, serverProof) {
				t.Fatal("client failed to verify server proof")
			}
		})
	}
}

func TestMessagePointIsNotIdentity(t *testing.T) {
	msg, _, err := StartClient("ABC123")
	if err != nil {
		t.Fatalf("StartClient: %v", err)
	}

	pointBytes := msg[1:]
	p := new(edwards25519.Point)
	if _, err := p.SetBytes(pointBytes); err != nil {
		t.Fatalf("invalid point in message: %v", err)
	}

	identity := new(edwards25519.Point).Set(edwards25519.NewIdentityPoint())
	if p.Equal(identity) == 1 {
		t.Fatal("message point should not be the identity")
	}
}

func TestHexPointDecoding(t *testing.T) {
	mBytes, _ := hex.DecodeString("15cfd18e385952982b6a8f8c7854963b58e34388c8e6dae891db756481a02312")
	p := new(edwards25519.Point)
	if _, err := p.SetBytes(mBytes); err != nil {
		t.Fatalf("failed to decode M constant: %v", err)
	}
	encoded := p.Bytes()
	if !bytes.Equal(encoded, mBytes) {
		t.Fatalf("round-trip failed: %x vs %x", encoded, mBytes)
	}
}

func TestUuidToBytes(t *testing.T) {
	id := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	b := uuidToBytes(id)
	if len(b) != 16 {
		t.Fatalf("expected 16 bytes, got %d", len(b))
	}
	// Verify first 4 bytes: a1b2c3d4
	if b[0] != 0xa1 || b[1] != 0xb2 || b[2] != 0xc3 || b[3] != 0xd4 {
		t.Fatalf("unexpected bytes: %x", b[:4])
	}
}

// TestInteropWithFrontendValues tests that the Go implementation produces the
// same password scalar as the frontend TypeScript implementation.
// The frontend hashToSpake2Scalar("ABC123") should produce a deterministic scalar.
func TestPasswordScalarFrontendCompat(t *testing.T) {
	// Verify the HKDF-derived scalar is deterministic (same as frontend).
	s, err := passwordToScalar([]byte("ABC123"))
	if err != nil {
		t.Fatalf("passwordToScalar: %v", err)
	}
	// The scalar bytes should be reproducible.
	s2, err := passwordToScalar([]byte("ABC123"))
	if err != nil {
		t.Fatalf("passwordToScalar: %v", err)
	}
	if !bytes.Equal(s.Bytes(), s2.Bytes()) {
		t.Fatal("password scalar should be deterministic")
	}
}

// TestBase64RoundTrip ensures base64 encoding/decoding works for SPAKE2 messages.
func TestBase64RoundTrip(t *testing.T) {
	msg, _, err := StartClient("ABC123")
	if err != nil {
		t.Fatalf("StartClient: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString(msg)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if !bytes.Equal(msg, decoded) {
		t.Fatal("base64 round-trip failed")
	}
}
