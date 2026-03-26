package league

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOracleWeekResults_Returns501 verifies the stub handler returns 501 Not Implemented.
func TestOracleWeekResults_Returns501(t *testing.T) {
	h := &Handler{} // service not needed for stub
	req := httptest.NewRequest(http.MethodPost, "/api/leagues/abc/oracle/week-results", nil)
	w := httptest.NewRecorder()

	h.OracleWeekResults(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected status 501, got %d", w.Code)
	}
}
