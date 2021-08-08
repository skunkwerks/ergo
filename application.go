package ergo

// http://erlang.org/doc/apps/kernel/application.html

import (
	"fmt"
	"sync"
	"time"

	"github.com/halturin/ergo/etf"
	"github.com/halturin/ergo/lib"
)

type ApplicationStartType = string

const (
	// start types:

	// ApplicationStartPermanent If a permanent application terminates,
	// all other applications and the runtime system (node) are also terminated.
	ApplicationStartPermanent = "permanent"

	// ApplicationStartTemporary If a temporary application terminates,
	// this is reported but no other applications are terminated.
	ApplicationStartTemporary = "temporary"

	// ApplicationStartTransient If a transient application terminates
	// with reason normal, this is reported but no other applications are
	// terminated. If a transient application terminates abnormally, that
	// is with any other reason than normal, all other applications and
	// the runtime system (node) are also terminated.
	ApplicationStartTransient = "transient"
)

// ApplicationBehavior interface
type ApplicationBehavior interface {
	ProcessBehavior
	Load(args ...etf.Term) (ApplicationSpec, error)
	Start(process *Process, args ...etf.Term)
}

type ApplicationSpec struct {
	Name         string
	Description  string
	Version      string
	Lifespan     time.Duration
	Applications []string
	Environment  map[string]interface{}
	// Depends		[]
	Children  []ApplicationChildSpec
	startType ApplicationStartType
	app       ApplicationBehavior
	process   *Process
	mutex     sync.Mutex
}

type ApplicationChildSpec struct {
	Child   ProcessBehavior
	Name    string
	Args    []etf.Term
	process *Process
}

// Application is implementation of ProcessBehavior interface
type Application struct{}

type ApplicationInfo struct {
	Name        string
	Description string
	Version     string
	PID         etf.Pid
}

func (a *Application) Loop(p *Process, args ...etf.Term) string {
	// some internal agreement that the first argument should be a spec of this application
	// (see ApplicatoinStart for the details)
	object := p.object
	spec := args[0].(*ApplicationSpec)
	p.SetTrapExit(true)

	if spec.Environment != nil {
		for k, v := range spec.Environment {
			p.SetEnv(k, v)
		}
	}

	if !a.startChildren(p, spec.Children[:]) {
		a.stopChildren(p.Self(), spec.Children[:], "failed")
		return "failed"
	}

	p.currentFunction = "Application:Start"

	object.(ApplicationBehavior).Start(p, args[1:]...)
	lib.Log("Application spec %#v\n", spec)
	p.ready <- nil

	p.currentFunction = "Application:loop"

	if spec.Lifespan == 0 {
		spec.Lifespan = time.Second * 31536000 * 100 // let's define default lifespan 100 years :)
	}

	// to prevent of timer leaks due to its not GCed until the timer fires
	timer := time.NewTimer(spec.Lifespan)
	defer timer.Stop()

	for {
		var message etf.Term

		select {
		case ex := <-p.gracefulExit:
			childrenStopped := a.stopChildren(ex.from, spec.Children, string(ex.reason))
			if !childrenStopped {
				fmt.Printf("Warining: application can't be stopped. Some of the children are still running")
				continue
			}
			return ex.reason

		case direct := <-p.direct:
			switch direct.id {
			case "$getChildren":
				pids := []etf.Pid{}
				for i := range spec.Children {
					if spec.Children[i].process == nil {
						continue
					}
					pids = append(pids, spec.Children[i].process.self)
				}

				direct.message = pids
				direct.err = nil
				direct.reply <- direct

			default:
				direct.message = nil
				direct.err = ErrUnsupportedRequest
				direct.reply <- direct
			}
			continue

		case <-p.Context.Done():
			// node is down or killed using p.Kill()
			return "kill"
		case <-timer.C:
			// time to die
			go p.Exit(p.Self(), "normal")
			continue
		case msg := <-p.mailBox:
			message = msg.message
		}

		//fromPid := msg.Element(1).(etf.Pid)
		switch r := message.(type) {
		case etf.Tuple:
			// waiting for {'EXIT', Pid, Reason}
			if len(r) != 3 || r.Element(1) != etf.Atom("EXIT") {
				// unknown. ignoring
				continue
			}
			terminated := r.Element(2).(etf.Pid)
			terminatedName := terminated.String()
			reason := r.Element(3).(etf.Atom)
			alienPid := true

			for i := range spec.Children {
				child := spec.Children[i].process
				if child == nil {
					continue
				}
				if child.Self() == terminated {
					terminatedName = child.Name()
					alienPid = false
					break
				}
			}

			// Application process has trapExit = true it means
			// the calling Exit methond will never send a message into
			// the gracefulExit channel, it will send an 'EXIT' message.
			// Checking bellow whether the terminated process was found
			// among the our children.
			if alienPid {
				// so we should proceed it as a graceful exit request and
				// terminate this Application process (if all children will
				// be stopped correctly)
				go func() {
					ex := gracefulExitRequest{
						from:   terminated,
						reason: string(reason),
					}
					p.gracefulExit <- ex
				}()
				continue
			}

			switch spec.startType {
			case ApplicationStartPermanent:
				a.stopChildren(terminated, spec.Children, string(reason))
				fmt.Printf("Application child %s (at %s) stopped with reason %s (permanent: node is shutting down)\n",
					terminatedName, p.Node.FullName, reason)
				p.Node.Stop()
				return "shutdown"

			case ApplicationStartTransient:
				if reason == etf.Atom("normal") || reason == etf.Atom("shutdown") {
					fmt.Printf("Application child %s (at %s) stopped with reason %s (transient)\n",
						terminatedName, p.Node.FullName, reason)
					continue
				}
				a.stopChildren(terminated, spec.Children, "normal")
				fmt.Printf("Application child %s (at %s) stopped with reason %s. (transient: node is shutting down)\n",
					terminatedName, p.Node.FullName, reason)
				p.Node.Stop()
				return string(reason)

			case ApplicationStartTemporary:
				fmt.Printf("Application child %s (at %s) stopped with reason %s (temporary)\n",
					terminatedName, p.Node.FullName, reason)
			}

		}

	}
}
func (a *Application) stopChildren(from etf.Pid, children []ApplicationChildSpec, reason string) bool {
	childrenStopped := true
	for i := range children {
		child := children[i].process
		if child == nil {
			continue
		}

		if child.self == from {
			continue
		}

		p := children[i].process
		if p == nil {
			continue
		}
		if !p.IsAlive() {
			continue
		}

		p.Exit(from, reason)
		if err := p.WaitWithTimeout(5 * time.Second); err != nil {
			childrenStopped = false
			continue
		}

		children[i].process = nil
	}

	return childrenStopped
}

func (a *Application) startChildren(parent *Process, children []ApplicationChildSpec) bool {
	for i := range children {
		// i know, it looks weird to use the funcion from supervisor file.
		// will move it to somewhere else, but let it be there for a while.
		p := startChild(parent, children[i].Name, children[i].Child, children[i].Args...)
		if p == nil {
			return false
		}
		children[i].process = p
	}
	return true
}
