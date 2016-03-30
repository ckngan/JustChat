package brokervec

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/rpc/jsonrpc"
	"sync"
	"time"

	"github.com/arcaneiceman/GoVector/broker/nonce"
)

/*
   This class manages the publishers connected to the broker and acts as an
   RPC server for them. It provides publishers with the ability to send
   messages to the broker that can be read by subscribers and written to a log
   file.

   The RPC calls are intended to be used by the wrapper provided in the govec
   library called GoPublisher.
*/

type PubManager struct {
	publishers    map[string]Publisher
	publishersMtx sync.Mutex // Mutex lock for preventing simultaneous access
	// to publishers map
	vb *VectorBroker

	listenPort string
}

// Create a new PubManager that listens on the port listenPort and sets up
// a tcp connection in a new GoRoutine
func NewPubManager(vb *VectorBroker, listenPort string) *PubManager {
	pm := &PubManager{
		publishers: make(map[string]Publisher),
		vb:         vb,
		listenPort: listenPort,
	}

	go pm.initPubManagerTCP()
	return pm
}

// Setup the pubmanager's TCP connection. Listens on pm.listenPort and when
// receiving a connection it registers the publisher and then serves a json
// RPC server over the connection in a new goroutine. This method blocks.
func (pm *PubManager) initPubManagerTCP() {
	port := ":" + pm.listenPort
	listener, e := net.Listen("tcp", port)
	if e != nil {
		log.Fatal("PubMgr: listen error:", e)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		name := pm.registerPublisher(conn)
		log.Println("PubMgr: Serving connection")
		go func() {
			jsonrpc.ServeConn(conn)
			pm.unregisterPublisher(name)
		}()
	}
}

// Registers a new publisher for providing information to the server
// sends the publisher's identifying name (a nonce) back over the net.Conn
// as a json object.
func (pm *PubManager) registerPublisher(conn net.Conn) (name string) {
	defer pm.publishersMtx.Unlock()

	var non *nonce.Nonce
	// Create a new nonce (will be based on the time registered)
	non = nonce.NewNonce("")
	// Use the nonce string as the name for the publisher
	name = non.Nonce
	pm.publishersMtx.Lock() //preventing simultaneous access to the `publishers` map
	if _, exists := pm.publishers[name]; exists {
		// Return error instead
		log.Panic("PubMgr: That publisher has already been registered, closing connection, please try again.")
		conn.Close()
		return
	}
	e := json.NewEncoder(conn)
	// Encode encodes and sends the object over the connection.
	tcperr := e.Encode(&non)

	if tcperr != nil {
		log.Fatal(tcperr)
	}

	publisher := TCPPub{
		Name: name,
		Conn: conn,
	}
	pm.publishers[name] = &publisher

	log.Println("PubMgr: " + name + " has joined the publisher list.")
	return name
}

// Unregisters a publisher
func (pm *PubManager) unregisterPublisher(name string) error {
	defer pm.publishersMtx.Unlock()

	pm.publishersMtx.Lock() //preventing simultaneous access to the `publishers` map
	if _, exists := pm.publishers[name]; exists {
		delete(pm.publishers, name)
		log.Println("PubMgr: Successfully unregistered: ", name)
	} else {
		log.Panic("PubMgr: Could not find that publisher.")
	}
	return nil
}

// Add a mesage to the message queue and return a reply or an error if failed.
func (pm *PubManager) AddMessage(msg Message, reply *string) (err error) {
	if _, exists := pm.publishers[msg.GetNonce()]; exists {
		pm.vb.AddMessage(msg)
		*reply = "Added to queue"
		err = nil
	} else {
		err = errors.New("PubMgr: Could not find that publisher.")
	}
	return err
}

// ****************
// RPC Calls
// ****************

//Adding local message to queue
func (pm *PubManager) AddLocalMsg(msg *LocalMessage, reply *string) error {
	log.Println("PubMgr: Adding local message from nonce: ", msg.GetNonce())
	msg.ReceiptTime = time.Now()
	err := pm.AddMessage(msg, reply)
	return err
}

//Adding network message to queue
func (pm *PubManager) AddNetworkMsg(msg *NetworkMessage, reply *string) error {
	log.Println("PubMgr: Adding net message from nonce: ", msg.GetNonce())
	msg.ReceiptTime = time.Now()
	err := pm.AddMessage(msg, reply)
	return err
}

// Function for processing a connection with a websocket.
//func (pm *PubManager) processFirstPublish(message string, conn *websocket.Conn) {

//    var non *nonce.Nonce
//    non = nonce.NewNonce(message)
//    err := websocket.JSON.Send(conn, non)
//    if err == nil {
//        fmt.Println("Sending nonce with nonce: ", non.Nonce)
//        pm.registerPublisher(non.Nonce, conn)
//    } else {
//        fmt.Println("Error creating nonce. Publisher not registered.")
//        conn.Close() //closing connection to indicate failed registration
//    }
//}
