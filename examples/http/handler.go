package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/halturin/ergo"
	"github.com/halturin/ergo/etf"
)

// GenServer implementation structure
type Handler struct {
	ergo.GenServer
}

type st struct {
	r *http.Request
}

type response struct {
	Request string
	Answer  string
}

// Init initializes process state using arbitrary arguments
// Init(...) -> state
func (h *Handler) Init(p *ergo.Process, args ...interface{}) (interface{}, error) {
	fmt.Println("Start handling http request")
	s := &st{
		r: args[0].(*http.Request),
	}
	return s, nil
}

func (h *Handler) HandleCast(message etf.Term, state ergo.GenServerState) string {
	fmt.Println(state.State.(*st).r.URL.Path)
	w := message.(http.ResponseWriter)
	w.Header().Set("Content-Type", "application/json")
	response := response{
		Request: state.State.(*st).r.URL.Path,
		Answer:  "This is answer for your request",
	}

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		fmt.Println(err)
	}
	return "stop"
}
