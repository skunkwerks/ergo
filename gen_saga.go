package ergo

import (
	"encoding/hex"
	"fmt"
	"math/rand"

	"github.com/halturin/ergo/etf"
)

type GenSaga struct {
	GenServer
}

type GenSagaTransactionOptions struct {
	// Name defines the name of this transaction. By default
	// this name has autogenerated ID.
	Name string
	// IgnoreLoop whether to cancel the transaction if a loop was detected.
	// Default is false.
	IgnoreLoop bool
	// HopLimit defines a number of hop within the transaction. Default limit
	// is 0 (no limit).
	HopLimit uint
}

type GenSagaOptions struct {
	// MaxTransactions defines the limit for the number of active transactions. Default: 0 (unlimited)
	MaxTransactions uint
}

type GenSagaState struct {
	GenServerState
	options GenSagaOptions
	txs     map[string]GenSagaTransaction
	State   interface{}
}

type GenSagaTransaction struct {
	Options GenSagaTransactionOptions
	Name    string
	Pid     etf.Pid
	Ref     etf.Ref
	parents []etf.Pid
}

// GenSagaBehavior interface
type GenSagaBehavior interface {
	//
	// Mandatory callbacks
	//

	// InitSaga
	InitSaga(state *GenSagaState, args ...interface{}) error

	// HandleCancel invoked on a request of transaction cancelation.
	HandleCancel(state *GenSagaState, tx GenSagaTransaction) error

	// HandleCanceled invoked if the given transaction has been canceled by some
	// reason (node or process went down or by explicit cancelation).
	HandleCanceled(state *GenSagaState, tx GenSagaTransaction, reason string) error

	// HandleDone
	HandleDone(state *GenSagaState, tx GenSagaTransaction, result interface{}) error

	// HandleTimeout
	HandleTimeout(state *GenSagaState, tx GenSagaTransaction, timeout int) error

	//
	// Optional callbacks
	//

	HandleNext(state *GenSagaState, tx GenSagaTransaction, arlg interface{}) error
	HandleInterim(state *GenSagaState, tx GenSagaTransaction, interim interface{}) error

	// HandleGenStageCall this callback is invoked on Process.Call. This method is optional
	// for the implementation
	HandleGenSagaCall(state *GenSagaState, from GenServerFrom, message etf.Term) (string, etf.Term)
	// HandleGenStageCast this callback is invoked on Process.Cast. This method is optional
	// for the implementation
	HandleGenSagaCast(state *GenSagaState, message etf.Term) string
	// HandleGenStageInfo this callback is invoked on Process.Send. This method is optional
	// for the implementation
	HandleGenSagaInfo(state *GenSagaState, message etf.Term) string
}

// default GenSaga callbacks

func (gs *GenSaga) HandleNext(state *GenSagaState, tx GenSagaTransaction, arg interface{}) error {
	fmt.Printf("HandleNext: unhandled message %#v\n", tx)
	return nil
}
func (gs *GenSaga) HandleCanceled(state *GenSagaState, tx GenSagaTransaction, reason string) error {
	// default callback if it wasn't implemented
	return nil
}
func (gs *GenSaga) HandleInterim(state *GenSagaState, tx GenSagaTransaction, interim interface{}) error {
	// default callback if it wasn't implemented
	fmt.Printf("HandleInterim: unhandled message %#v\n", tx)
	return nil
}

func (gs *GenSaga) HandleGenSagaCall(state *GenSagaState, from GenServerFrom, message etf.Term) (string, etf.Term) {
	// default callback if it wasn't implemented
	fmt.Printf("HandleGenSagaCall: unhandled message (from %#v) %#v\n", from, message)
	return "reply", etf.Atom("ok")
}

func (gs *GenSaga) HandleGenSagaCast(state *GenSagaState, message etf.Term) string {
	// default callback if it wasn't implemented
	fmt.Printf("HandleGenSagaCast: unhandled message %#v\n", message)
	return "noreply"
}
func (gs *GenSaga) HandleGenSagaInfo(state *GenSagaState, message etf.Term) string {
	// default callback if it wasn't implemnted
	fmt.Printf("HandleGenSagaInfo: unhandled message %#v\n", message)
	return "noreply"
}

//
// GenServer callbacks
//
func (gs *GenSaga) Init(state *GenServerState, args ...interface{}) error {
	sagaState := &GenSagaState{
		GenServerState: *state,
	}
	if err := state.Process.GetObject().(GenSagaBehavior).InitSaga(sagaState, args...); err != nil {
		return err
	}
	return nil
}

func (gs *GenSaga) HandleCall(state *GenServerState, from GenServerFrom, message etf.Term) (string, etf.Term) {
	return "reply", "ok"
}

func (gs *GenSaga) HandleCast(state *GenServerState, message etf.Term) string {
	return "noreply"
}

func (gs *GenSaga) HandleInfo(state *GenServerState, message etf.Term) string {
	return "noreply"
}

//
// private functions
//

func randomString(length int) string {
	buff := make([]byte, length)
	rand.Read(buff)
	return hex.EncodeToString(buff)
}
