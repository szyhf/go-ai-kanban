package api

import (
	"crypto/rand"
	"net/http"
)

const (
	enrollmentCodeLength = 6
	enrollmentCharset    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// handleEnrollmentCode handles POST /api/relay-auth/server/enrollment-code.
func (s *Server) handleEnrollmentCode(w http.ResponseWriter, r *http.Request) {
	code, err := generateEnrollmentCode()
	if err != nil {
		fail(w, "failed to generate enrollment code", http.StatusInternalServerError)
		return
	}

	if err := s.store.CreateEnrollmentCode(code); err != nil {
		fail(w, "failed to store enrollment code", http.StatusInternalServerError)
		return
	}

	success(w, map[string]string{
		"enrollment_code": code,
	})
}

func generateEnrollmentCode() (string, error) {
	b := make([]byte, enrollmentCodeLength)
	for i := range b {
		buf := make([]byte, 1)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		b[i] = enrollmentCharset[int(buf[0])%len(enrollmentCharset)]
	}
	return string(b), nil
}
