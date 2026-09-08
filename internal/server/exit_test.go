package server

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestApplicationExitAcknowledgesBeforeShutdown(t *testing.T) {
	exited := make(chan struct{})
	w := httptest.NewRecorder()
	applicationExitHandler(func() { close(exited) })(w, httptest.NewRequest("POST", "/application/exit", nil))
	if w.Code != 200 || !w.Flushed || w.Body.String() != `{"ok":true}` {
		t.Fatal("Exit request was not acknowledged")
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("Shutdown callback was not called")
	}
}
