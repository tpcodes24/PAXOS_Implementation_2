package paxos

//
// Paxos library, to be included in an application.
// Multiple applications will run, each including
// a Paxos peer.
//
// Manages a sequence of agreed-on values.
// The set of peers is fixed.
// Copes with network failures (partition, msg loss, &c).
// Does not store anything persistently, so cannot handle crash+restart.
//
// The application interface:
//
// px = paxos.Make(peers []string, me string)
// px.Start(seq int, v interface{}) -- start agreement on new instance
// px.Status(seq int) (Fate, v interface{}) -- get info about an instance
// px.Done(seq int) -- ok to forget all instances <= seq
// px.Max() int -- highest instance seq known, or -1
// px.Min() int -- instances before this seq have been forgotten
//

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"reflect"
	"strconv"

	// "strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// px.Status() return values, indicating
// whether an agreement has been decided,
// or Paxos has not yet reached agreement,
// or it was agreed but forgotten (i.e. < Min()).
type Fate int

const (
	Decided   Fate = iota + 1
	Pending        // not yet decided.
	Forgotten      // decided but forgotten.
)

type Paxos struct {
	mu                sync.Mutex
	l                 net.Listener
	dead              int32 // for testing
	unreliable        int32 // for testing
	rpcCount          int32 // for testing
	peers             []string
	me                int // index into peers[]
	instances         map[int]*InstanceRecord
	MinproposalNumber string
	done              []int
	// Your data here.
}

type PrepareArgs struct {
	Seq      int    // Sequence number of the instance
	Proposal string // Proposal number
	ServerId int
	Done     int
}

type PrepareReply struct {
	Err bool
	N_a string // Highest proposal number accepted
	V_a interface{}
}

type InstanceRecord struct {
	ProposalNumber         string
	AcceptedValue          interface{}
	Status                 Fate
	AcceptedProposalNumber string
}

type AcceptArgs struct {
	Seq            int // Sequence number of the instance
	ProposalNumber string
	ProposalValue  interface{} // Proposed value
	Done           int         // Value accepted
}

type AcceptReply struct {
	ProposalNumber string
	Err            bool
}

type DecideArgs struct {
	Seq          int
	DecidedValue interface{}
	ProposalNum  string
	ServerId     int
	Done         int
}

type DecideReply struct {
	Err bool
}

func convertStringToFloat(s string) (float64, error) {
	// Attempt to convert the string to a floating-point number.

	if s == "" {
		return 0.0, nil
	}
	// fmt.Println(s, "ghxhgchhvhjhjjjkj")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0, err // Handle the error, e.g., return 0.0 or propagate the error.
	}

	return f, nil
}

func convertStringToInt(s string) (int, error) {
	if s == "" {
		// Handle the case where the string is empty.
		return 0, nil
	}

	// Attempt to convert the string to an integer.
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, err // Handle the error, e.g., return 0 or propagate the error.
	}

	return i, nil
}

// call() sends an RPC to the rpcname handler on server srv
// with arguments args, waits for the reply, and leaves the
// reply in reply. the reply argument should be a pointer
// to a reply structure.
//
// the return value is true if the server responded, and false
// if call() was not able to contact the server. in particular,
// the replys contents are only valid if call() returned true.
//
// you should assume that call() will time out and return an
// error after a while if it does not get a reply from the server.
//
// please use call() to send all RPCs, in client.go and server.go.
// please do not change this function.
func call(srv string, name string, args interface{}, reply interface{}) bool {
	// fmt.Println(srv, name, args)
	c, err := rpc.Dial("unix", srv)
	if err != nil {
		err1 := err.(*net.OpError)
		if err1.Err != syscall.ENOENT && err1.Err != syscall.ECONNREFUSED {
			fmt.Printf("paxos Dial() failed: %v\n", err1)
		}
		return false
	}
	defer c.Close()

	err = c.Call(name, args, reply)
	// fmt.Println(srv, name, reply, "reply")

	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}

// func (px *Paxos) generateProposalNumber() string {
// 	roundAndserver := px.proposalNumber
// 	// Split the string based on the decimal point
// 	parts := strings.Split(roundAndserver, ".")

// 	// Extract the integer part before the decimal point
// 	round := parts[0]
// 	serverId := px.me

// 	// Convert the integer string to an actual integer
// 	roundInt, err := convertStringToInt(round)
// 	if err != nil {
// 		fmt.Println("Error converting string to integer:", err)
// 		return "error converting string to integer"
// 	}
// 	roundInt += 1
// 	// Concatenate the integers with a decimal point
// 	combinedString := strconv.Itoa(roundInt) + "." + strconv.Itoa(serverId)

// 	// Print the combined string
// 	// fmt.Println("Combined string:", combinedString)
// 	px.proposalNumber = combinedString
// 	return combinedString

// }

func (px *Paxos) Prepare(args *PrepareArgs, reply *PrepareReply) error {
	px.mu.Lock()
	defer px.mu.Unlock()
	if _, exists := px.instances[args.Seq]; !exists {
		px.instances[args.Seq] = &InstanceRecord{
			ProposalNumber:         "",
			AcceptedValue:          nil,
			Status:                 Pending,
			AcceptedProposalNumber: "",
		}
	}
	instance := px.instances[args.Seq]

	if args.Proposal > instance.ProposalNumber {
		reply.Err = true
		reply.N_a = instance.AcceptedProposalNumber
		reply.V_a = instance.AcceptedValue
		instance.ProposalNumber = args.Proposal
		instance.AcceptedProposalNumber = args.Proposal
	} else {
		reply.Err = false
		// reply.N_a=instance.AcceptedProposalNumber
		// reply.V_a=instance.AcceptedValue
	}

	// acceptor's prepare(n) handler:
	// if n > n_p
	//     n_p = n
	//     reply prepare_ok(n, n_a, v_a)
	// else
	//     reply prepare_reject
	// fmt.Println("hello100fffffffff")
	// fmt.Println(args.serverId, px.done, "PREPAREEEEEE")

	// instance.ProposalNumber
	// num1, err1 := convertStringToFloat(instance.ProposalNumber)
	// // fmt.Println("hello109hhhhhhhhhh")
	// // args.Proposal
	// num2, err2 := convertStringToFloat(args.Proposal)
	// if err1 != nil {
	// 	fmt.Println("Error:", err1)
	// 	return err1
	// }
	// if err2 != nil {
	// 	fmt.Println("Error:", err2)
	// 	return err2
	// }
	// if num2 > num1 {
	// 	// This proposal is higher, so we update our state and respond with accepted data.
	// 	reply.Err = true
	// 	reply.N_a = instance.ProposalNumber
	// 	reply.V_a = instance.ProposalValue

	// } else {
	// 	// This proposal is not higher, so we reject it.
	// 	reply.Err = false
	// 	reply.N_a = instance.ProposalNumber
	// 	reply.V_a = instance.ProposalValue
	// }

	return nil
}

// Accept handles the acceptance phase of Paxos.
func (px *Paxos) Accept(args *AcceptArgs, reply *AcceptReply) error {
	px.mu.Lock()
	defer px.mu.Unlock()
	// fmt.Println(px.me, args, reply)
	if _, exists := px.instances[args.Seq]; !exists {
		px.instances[args.Seq] = &InstanceRecord{
			ProposalNumber:         "",
			AcceptedValue:          nil,
			Status:                 Pending,
			AcceptedProposalNumber: "",
		}
	}
	instance := px.instances[args.Seq]

	if args.ProposalNumber >= instance.ProposalNumber {
		// This proposal is higher or equal, so we update our state and accept the value.
		instance.ProposalNumber = args.ProposalNumber
		instance.AcceptedProposalNumber = args.ProposalNumber
		instance.AcceptedValue = args.ProposalValue
		reply.Err = true
	} else {
		reply.Err = false
	}

	// fmt.Printf("hello2")
	// doneValue := px.done[px.me]
	// num1, err1 := convertStringToFloat(instance.ProposalNumber)
	// // fmt.Printf("hello3")

	// num2, err2 := convertStringToFloat(args.ProposalNumber)
	// if err1 != nil {
	// 	fmt.Println("Error:", err1)
	// 	return err1
	// }
	// if err2 != nil {
	// 	fmt.Println("Error:", err2)
	// 	return err2
	// }

	// if num2 >= num1 {
	// 	// This proposal is higher or equal, so we update our state and accept the value.
	// 	instance.ProposalNumber = args.ProposalNumber
	// 	instance.ProposalValue = args.ProposalValue
	// 	instance.AcceptedProposalNumber = args.ProposalNumber
	// 	reply.Err = true

	// } else {
	// 	// This proposal is lower, so we reject it.
	// 	reply.Err = false

	// }

	return nil
}

func (px *Paxos) Decided(args *DecideArgs, reply *DecideReply) error {
	// fmt.Println(args)
	px.mu.Lock()
	defer px.mu.Unlock()

	if _, exists := px.instances[args.Seq]; !exists {
		px.instances[args.Seq] = &InstanceRecord{
			ProposalNumber:         "",
			AcceptedValue:          nil,
			Status:                 Pending,
			AcceptedProposalNumber: "",
		}
	}
	instance := px.instances[args.Seq]

	instance.Status = Decided
	instance.AcceptedProposalNumber = args.ProposalNum
	instance.AcceptedValue = args.DecidedValue
	instance.ProposalNumber = args.ProposalNum
	px.done[args.ServerId] = args.Done
	// fmt.Println(px.done)
	// fmt.Println(instance, args.DecidedValue, instance.AcceptedValue)

	reply.Err = true

	return nil
}

// the application wants paxos to start agreement on
// instance seq, with proposed value v.
// Start() returns right away; the application will
// call Status() to find out if/when agreement
// is reached.
func IsNilish(val any) bool {
	if val == nil {
		return true
	}

	v := reflect.ValueOf(val)
	k := v.Kind()
	switch k {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer,
		reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return v.IsNil()
	}

	return false
}
func (px *Paxos) Start(seq int, v interface{}) {
	// Your code here.
	// fmt.Println("START PROPOSING", seq, v, px.me)
	go func() {
		if seq < px.Min() {
			return
		}
		if state, _ := px.Status(seq); state == Decided {
			return
		}
		for {

			// Choose a unique and higher proposal number 'n'.
			// newProposalNumber := px.generateProposalNumber()
			newProposalNumber := strconv.FormatInt(time.Now().UnixNano(), 10) + "#" + strconv.Itoa(px.me)
			// if _, exists := px.instances[seq]; !exists {
			// 	px.instances[seq] = &InstanceRecord{
			// 		ProposalNumber:            "",
			// 		AcceptedValue:             nil,
			// 		Status:                    Pending,
			// 		AcceptedProposalNumber: "",
			// 	}
			// }

			// record := px.instances[seq]

			// Send prepare(n) to all peers, including self.
			prepareOKCount := 0
			maxAcceptedN := ""
			acceptedValue := v

			for i := range px.peers {
				reply := PrepareReply{}

				// Send prepare RPC to peer i.
				if i == px.me {
					px.Prepare(&PrepareArgs{seq, newProposalNumber, px.me, px.done[px.me]}, &reply)
					if reply.Err == true {
						prepareOKCount++
						// _, err1 := convertStringToFloat(reply.N_a)
						// _, err2 := convertStringToFloat(maxAcceptedN)
						// if err1 != nil {
						// 	fmt.Println("Error:", err1)
						// }
						// if err2 != nil {
						// 	fmt.Println("Error:", err2)
						// }

						if reply.N_a > maxAcceptedN && (reply.V_a != nil) {
							maxAcceptedN = reply.N_a
							acceptedValue = reply.V_a
						}
					}
				} else {
					if call(px.peers[i], "Paxos.Prepare", PrepareArgs{seq, newProposalNumber, px.me, px.done[px.me]}, &reply) {
						if reply.Err == true {
							prepareOKCount++
							// _, err1 := convertStringToFloat(reply.N_a)
							// _, err2 := convertStringToFloat(maxAcceptedN)
							// if err1 != nil {
							// 	fmt.Println("Error:", err1)
							// }
							// if err2 != nil {
							// 	fmt.Println("Error:", err2)
							// }
							// fmt.Println(px.me, reply, i, reply.N_a, reply.V_a, maxAcceptedN, acceptedValue, reply.V_a == nil, !(reply.V_a == nil), reply.N_a > maxAcceptedN && !(reply.V_a == nil))
							// c := reply.N_a
							if reply.N_a > maxAcceptedN && !(reply.V_a == nil) {
								maxAcceptedN = reply.N_a
								acceptedValue = reply.V_a
							}
							// fmt.Println(maxAcceptedN, acceptedValue, "after")
						}
					}
				}
			}

			// If prepare_ok(n) from majority:
			if prepareOKCount >= len(px.peers)/2+1 {
				// Determine the value to propose.
				// if acceptedValue != nil {
				// 	v = acceptedValue
				// }

				// Send accept(n, v) to all peers.
				acceptOKCount := 0
				// breakk := false

				for i := range px.peers {
					reply := AcceptReply{}

					if i == px.me {
						// Handle accept from self.
						px.Accept(&AcceptArgs{seq, newProposalNumber, acceptedValue, px.done[px.me]}, &reply)
						if reply.Err == true {
							acceptOKCount++
						}

						// if reply.ProposalNumber > px {
						// 	breakk = true
						// }
					} else {
						// Send accept RPC to peer i.
						if call(px.peers[i], "Paxos.Accept", AcceptArgs{seq, newProposalNumber, acceptedValue, px.done[px.me]}, &reply) {
							if reply.Err == true {
								acceptOKCount++
							}

							// if reply.ProposalNumber > px.proposalNumber {
							// 	breakk = true
							// }
						}
					}
				}

				// if breakk {
				// 	continue
				// }

				// If accept_ok(n) from majority:
				if acceptOKCount >= len(px.peers)/2+1 {
					// var dArgs DecideArgs
					// dArgs.Seq = seq
					// dArgs.serverId = px.me
					// dArgs.done = px.done[px.me]
					// Send decided(v) to all peers.
					decidedArgsSent := &DecideArgs{Seq: seq, ProposalNum: newProposalNumber, DecidedValue: acceptedValue, ServerId: px.me, Done: px.done[px.me]}
					for i := range px.peers {
						reply := DecideReply{}

						if i == px.me {
							// Handle decided from self.
							px.Decided(decidedArgsSent, &reply)
						} else {
							// Send decided RPC to peer i.
							call(px.peers[i], "Paxos.Decided", decidedArgsSent, &reply)
						}
					}
					// break

					break
				}
			}

		}
	}()
}

// the application on this machine is done with
// all instances <= seq.
//
// see the comments for Min() for more explanation.
func (px *Paxos) Done(seq int) {
	// Your code here.
	px.mu.Lock()
	defer px.mu.Unlock()
	// fmt.Println("DONNNEEEE", seq, px.me, px.instances)
	if seq > px.done[px.me] {
		px.done[px.me] = seq

	}

}

// the application wants to know the
// highest instance sequence known to
// this peer.
func (px *Paxos) Max() int {
	px.mu.Lock()
	defer px.mu.Unlock()
	maxSeq := -1

	// Iterate through instances to find the maximum sequence.
	for seq := range px.instances {
		if seq > maxSeq {
			maxSeq = seq
		}
	}

	return maxSeq
}

// Min() should return one more than the minimum among z_i,
// where z_i is the highest number ever passed
// to Done() on peer i. A peers z_i is -1 if it has
// never called Done().
//
// Paxos is required to have forgotten all information
// about any instances it knows that are < Min().
// The point is to free up memory in long-running
// Paxos-based servers.
//
// Paxos peers need to exchange their highest Done()
// arguments in order to implement Min(). These
// exchanges can be piggybacked on ordinary Paxos
// agreement protocol messages, so it is OK if one
// peers Min does not reflect another Peers Done()
// until after the next instance is agreed to.
//
// The fact that Min() is defined as a minimum over
// all Paxos peers means that Min() cannot increase until
// all peers have been heard from. So if a peer is dead
// or unreachable, other peers Min()s will not increase
// even if all reachable peers call Done. The reason for
// this is that when the unreachable peer comes back to
// life, it will need to catch up on instances that it
// missed -- the other peers therefor cannot forget these
// instances.
func (px *Paxos) Min() int {
	// You code here.
	px.mu.Lock()
	defer px.mu.Unlock()
	minSeq := px.done[px.me]
	// fmt.Println(px.done, px.me, "MINNN") ./
	// Iterate through all peers' Done sequence numbers to find the minimum.

	for i := range px.done {
		if px.done[i] < minSeq {
			minSeq = px.done[i]
		}
	}
	// for _, doneSeq := range px.done {
	// 	if doneSeq < minSeq {
	// 		minSeq = doneSeq
	// 	}
	// }

	// Add 1 to the minimum to get the value required by the contract.
	// Call Min to determine the new Min sequence number.

	// Remove any instances with sequence numbers less than the new Min.
	// for seq := range px.instances {
	// 	if seq <= minSeq {
	// 		delete(px.instances, seq)
	// 	}
	// }
	for i, _ := range px.instances {
		if i <= minSeq {
			delete(px.instances, i)
		}
	}
	return minSeq + 1
	// return 0
}

// the application wants to know whether this
// peer thinks an instance has been decided,
// and if so what the agreed value is. Status()
// should just inspect the local peer state;
// it should not contact other Paxos peers.
func (px *Paxos) Status(seq int) (Fate, interface{}) {

	// Check if the instance has been decided.
	// You need to implement your logic here to determine the status.
	mini := px.Min()
	px.mu.Lock()
	defer px.mu.Unlock()
	if seq < mini {
		return Forgotten, nil
	}

	if instance, ok := px.instances[seq]; ok {
		return instance.Status, instance.AcceptedValue
	} else {
		return Pending, nil
	}

}

// tell the peer to shut itself down.
// for testing.
// please do not change these two functions.
func (px *Paxos) Kill() {
	atomic.StoreInt32(&px.dead, 1)
	if px.l != nil {
		px.l.Close()
	}
}

// has this peer been asked to shut down?
func (px *Paxos) isdead() bool {
	return atomic.LoadInt32(&px.dead) != 0
}

// please do not change these two functions.
func (px *Paxos) setunreliable(what bool) {
	if what {
		atomic.StoreInt32(&px.unreliable, 1)
	} else {
		atomic.StoreInt32(&px.unreliable, 0)
	}
}

func (px *Paxos) isunreliable() bool {
	return atomic.LoadInt32(&px.unreliable) != 0
}

// the application wants to create a paxos peer.
// the ports of all the paxos peers (including this one)
// are in peers[]. this servers port is peers[me].
func Make(peers []string, me int, rpcs *rpc.Server) *Paxos {
	px := &Paxos{}
	px.peers = peers
	px.me = me

	// Your initialization code here.
	px.instances = make(map[int]*InstanceRecord)
	// px.done = make([]int, )
	px.done = make([]int, len(px.peers))
	for i := range px.done {
		px.done[i] = -1
	}
	if rpcs != nil {
		// caller will create socket &c
		rpcs.Register(px)
	} else {
		rpcs = rpc.NewServer()
		rpcs.Register(px)

		// prepare to receive connections from clients.
		// change "unix" to "tcp" to use over a network.
		os.Remove(peers[me]) // only needed for "unix"
		l, e := net.Listen("unix", peers[me])
		if e != nil {
			log.Fatal("listen error: ", e)
		}
		px.l = l

		// please do not change any of the following code,
		// or do anything to subvert it.

		// create a thread to accept RPC connections
		go func() {
			for px.isdead() == false {
				conn, err := px.l.Accept()
				if err == nil && px.isdead() == false {
					if px.isunreliable() && (rand.Int63()%1000) < 100 {
						// discard the request.
						conn.Close()
					} else if px.isunreliable() && (rand.Int63()%1000) < 200 {
						// process the request but force discard of reply.
						c1 := conn.(*net.UnixConn)
						f, _ := c1.File()
						err := syscall.Shutdown(int(f.Fd()), syscall.SHUT_WR)
						if err != nil {
							fmt.Printf("shutdown: %v\n", err)
						}
						atomic.AddInt32(&px.rpcCount, 1)
						go rpcs.ServeConn(conn)
					} else {
						atomic.AddInt32(&px.rpcCount, 1)
						go rpcs.ServeConn(conn)
					}
				} else if err == nil {
					conn.Close()
				}
				if err != nil && px.isdead() == false {
					fmt.Printf("Paxos(%v) accept: %v\n", me, err.Error())
				}
			}
		}()
	}

	return px
}
