package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arcaneiceman/GoVector/govec"
)

/*
	----DECLARED TYPES----
*/
//RPC Value for recieving messages
type NodeService int
type MessageService int

// Reply
type ValReply struct {
	Val string // value; depends on the call
}

type ServerReply struct {
	Message string // value; depends on the call
}

type NodeListReply struct {
	ListOfNodes *ServerItem
}

type ServerItem struct {
	UDP_IPPORT        string
	RPC_CLIENT_IPPORT string
	RPC_SERVER_IPPORT string
	Clients           int
	NextServer        *ServerItem
}

//NewStorageNode Args
type NewNodeSetup struct {
	RPC_CLIENT_IPPORT string
	RPC_SERVER_IPPORT string
	UDP_IPPORT        string
}

// Message Format from client
type ClientMessage struct {
	Username string
	Message  string
}

// Clock stamped client message for ordered chat history
type ClockedClientMsg struct {
	ClientMsg ClientMessage
	ServerId  string
	Clock     int
}

type ClientRequest struct {
	Username string // requesting info for this client
}

// FileInfoData to build file structure in rpc call
type FileData struct {
	Username string
	FileName string
	FileSize int64
	Data     []byte
}

type FileInfo struct {
	Username string
	FileName string
}

// FileInfoData to build file structure in rpc call
type StoreFileData struct {
	Username   string
	UDP_IPPORT string
	FileName   string
	FileSize   int64
	Data       []byte
}

//Client object
type ClientItem struct {
	Username   string
	RPC_IPPORT string
	NextClient *ClientItem
}

//cient info
type ClientInfo struct {
	Username   string
	RPC_IPPORT string
}

// load balancer types
type NodeToRemove struct {
	Node *ServerItem
}

type LBReply struct {
	Message string
}

type BackService int

//GLOBALS
var LOAD_BALANCER_IPPORT string
var SEND_PING_IPPORT string
var RECEIVE_PING_ADDR string
var RPC_SYSTEM_IPPORT string
var RPC_CLIENT_IPPORT string
var serverList *ServerItem
var serverListMutex *sync.Mutex
var clientList *ClientItem
var clientListMutex *sync.Mutex
var thisClock int                   // number of messages received from own clients
var numMsgsRcvd int                 // # of messages this node has received
var toHistoryBuf []ClockedClientMsg // temp storage for messages before disk write
var historyMutex *sync.Mutex
var Logger *govec.GoLog // GoVector log

//****************************BACK-END RPC METHODS***********************************//
func (nodeSvc *NodeService) NewStorageNode(args *NewNodeSetup, reply *ServerReply) error {
	println("A new server node has joined the system")
	Logger.LogLocalEvent("new server node acknowledged")
	addNode(args.UDP_IPPORT, args.RPC_CLIENT_IPPORT, args.RPC_SERVER_IPPORT)
	reply.Message = "success"
	return nil
}

func (nodeSvc *NodeService) SendPublicMsg(args *ClockedClientMsg, reply *ServerReply) error {
	historyMutex.Lock()
	println("we received a new message")
	Logger.LogLocalEvent("received new public message")

	inClockedMsg := ClockedClientMsg{
		ClientMsg: args.ClientMsg,
		ServerId:  args.ServerId,
		Clock:     args.Clock}

	numMsgsRcvd++
	toHistoryBuf[numMsgsRcvd-1] = inClockedMsg

	if thisClock < numMsgsRcvd {
		thisClock = numMsgsRcvd
	}

	sendPublicMsgClients(inClockedMsg.ClientMsg)

	checkBufFull()
	historyMutex.Unlock()

	reply.Message = "success"
	return nil
}

func (nodeSvc *NodeService) SendPublicFile(args *FileData, reply *ServerReply) error {
	println("We received a new File")
	Logger.LogLocalEvent("received new public file")

	file := FileData{
		Username: args.Username,
		FileName: args.FileName,
		FileSize: args.FileSize,
		Data:     args.Data}

	sendPublicFileClients(file)

	reply.Message = "success"
	return nil
}

func (nodeSvc *NodeService) StoreFile(args *FileData, reply *ServerReply) error {
	println("Storing A File...")
	Logger.LogLocalEvent("storing a file")

	file := FileData{
		Username: args.Username,
		FileName: args.FileName,
		FileSize: args.FileSize,
		Data:     args.Data}
	storeFile(file)

	reply.Message = "success"
	return nil
}

func (nodeSvc *NodeService) GetFile(filename *string, reply *FileData) error {
	Logger.LogLocalEvent("received file request")
	path := "../Files/" + *filename

	fi, err := os.Stat(path)

	if os.IsNotExist(err) {
		reply.Username = "404"

	} else {
		// re-open file
		var file, errr = os.OpenFile(path, os.O_RDWR, 0644)
		checkError(errr)
		defer file.Close()

		Data := make([]byte, fi.Size())

		_, _ = file.Read(Data)

		checkError(err)
		reply.Username = "202"
		reply.FileName = *filename
		reply.FileSize = fi.Size()
		reply.Data = Data
		Logger.LogLocalEvent("sending requested file")
	}

	return nil
}

func (nodeSvc *NodeService) DeleteFile(args *FileData, reply *ServerReply) error {
	Logger.LogLocalEvent("received delete file request")
	println("Deleting file: ", args.FileName)

	path := "../Files/" + args.FileName

	// detect if file exists
	_, err := os.Stat(path)

	// create file if not exists
	if os.IsNotExist(err) {
		reply.Message = "File " + path + " Doesn't Exist"
	} else {
		err = os.Remove(path)
		checkError(err)
		reply.Message = "success"
	}

	return nil
}

//***********************CLIENT RPC METHODS **********************************************//
//method for joining the storage node
func (msgSvc *MessageService) ConnectionInit(message *ClientInfo, reply *ServerReply) error {
	println("A client has joined the server.")
	addClient(message.Username, message.RPC_IPPORT)
	reply.Message = "success"
	return nil
}

// method for public message transfer
func (ms *MessageService) SendPublicMsg(args *ClientMessage, reply *ServerReply) error {
	historyMutex.Lock()
	println("Message from client received: ", args.Message)

	message := ClientMessage{
		Username: args.Username,
		Message:  args.Message}

	thisClock++
	numMsgsRcvd++

	var hinder sync.WaitGroup
	hinder.Add(2)
	go func() {
		defer hinder.Done()
		sendPublicMsgServers(message)
	}()
	go func() {
		defer hinder.Done()
		sendPublicMsgClients(message)
	}()

	checkBufFull()
	historyMutex.Unlock()

	hinder.Wait()
	reply.Message = "success"
	return nil
}

// method for public file transfer
func (ms *MessageService) SendPublicFile(args *FileData, reply *ServerReply) error {
	println("File Received.")

	file := FileData{
		Username: args.Username,
		FileName: args.FileName,
		FileSize: args.FileSize,
		Data:     args.Data}

	var hinder sync.WaitGroup
	hinder.Add(2)
	go func() {
		defer hinder.Done()
		sendPublicFileServers(file)
	}()
	go func() {
		defer hinder.Done()
		sendPublicFileClients(file)
	}()
	//TODO: UNCOMMENT THIS!!!!!! It is commented out for testning
	//storeFile(file)

	//Send LB Filename to LB
	var rep string
	systemService, err := rpc.Dial("tcp", LOAD_BALANCER_IPPORT)
	checkError(err)
	err = systemService.Call("NodeService.NewFile", args.FileName, &rep)
	checkError(err)
	println(rep)
	systemService.Close()

	hinder.Wait()
	reply.Message = "success"
	return nil
}

// Method to request client information for private correspondence
func (ms *MessageService) SendPrivate(args *ClientRequest, reply *ClientInfo) error {
	println("username requested: " + args.Username)
	//find requested user's IP and send it back
	rep := getAddr(args.Username)
	reply.Username = args.Username
	reply.RPC_IPPORT = rep

	return nil
}

func (ms *MessageService) GetFile(filename *string, reply *FileData) error {

	serverListMutex.Lock()
	next := serverList
	serverListMutex.Unlock()

	for next != nil {

		if (*next).UDP_IPPORT != RECEIVE_PING_ADDR {

			systemService, err := rpc.Dial("tcp", (*next).RPC_SERVER_IPPORT)
			//checkError(err)
			if err != nil {
				println("SendPublicMsg To Servers: Server ", (*next).UDP_IPPORT, " isn't accepting tcp conns so skip it...")
				reply.Username = "404"
			} else {
				var rep FileData
				err = systemService.Call("NodeService.GetFile", *filename, &rep)
				checkError(err)
				if err == nil && rep.Username != "404" {
					fmt.Println("sent file to client: ", rep.FileName)
					reply.Username = "202"
					reply.FileName = rep.FileName
					reply.FileSize = rep.FileSize
					reply.Data = rep.Data
					break
				} else {
					reply.Username = "404"
				}
				systemService.Close()
			}
		} else {
			println("our server has it")
			resp := possessFile(*filename)
			if resp.Username != "404" {
				reply.Username = resp.Username
				reply.FileName = resp.FileName
				reply.FileSize = resp.FileSize
				reply.Data = resp.Data
				break
			} else {
				reply.Username = "404"
			}
		}
		next = (*next).NextServer
	}

	return nil
}

//***********************Load Balancer RPC METHODS **********************************************//
//method for deleting a dead storage node
func (lbServ *NodeService) RemoveNode(nodeToRemove *NodeToRemove, callback *LBReply) error {
	//When recieve notice of a dead node (Lock access to serverlist and remove the dead node)
	serverListMutex.Lock()
	println("\n\nCall to delete")
	deleteNodeFromList(nodeToRemove.Node.UDP_IPPORT)
	println("Should be deleted")
	Logger.LogLocalEvent("node removed")
	serverListMutex.Unlock()
	return nil
}

func main() {
	// PARSE ARGS
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr,
			"Usage: %s [loadbalancer ip:port1] [udp_ping ip:port2]\n",
			os.Args[0])
		os.Exit(1)
	}

	LOAD_BALANCER_IPPORT = os.Args[1]
	SEND_PING_IPPORT = os.Args[2]
	println("LOAD_BALANCER: ", LOAD_BALANCER_IPPORT, " SEND_PINGS: ", SEND_PING_IPPORT)

	serverListMutex = &sync.Mutex{}
	clientListMutex = &sync.Mutex{}
	clientList = nil

	// setup for chat history
	toHistoryBuf = make([]ClockedClientMsg, 50)
	historyMutex = &sync.Mutex{}
	thisClock = 0
	numMsgsRcvd = 0

	// Create log
	Logger = govec.InitializeMutipleExecutions("server "+RECEIVE_PING_ADDR, "sys")
	Logger.LogThis("server was initialized", "server "+RECEIVE_PING_ADDR, "{\"server "+RECEIVE_PING_ADDR+"\":1}")

	////////////////////////////////////////////////////////////////////////////////////////

	// LOAD BALANCER tcp.rpc
	ip := "localhost" //getIP()
	nodeService := new(NodeService)
	rpc.Register(nodeService)
	c := make(chan int)
	go func() {
		systemListenServe(ip+":0", c)
	}()
	RPC_system_port := <-c
	RPC_SYSTEM_IPPORT = ip + ":" + strconv.Itoa(RPC_system_port)
	println("RPC PORT FOR SYSTEMS: " + RPC_SYSTEM_IPPORT)

	//CLIENT tcp.rpc
	messageService := new(MessageService)
	rpc.Register(messageService)
	ch := make(chan int)
	go func() {
		clientListenServe(ip+":0", ch)
	}()
	RPC_client_port := <-ch
	RPC_CLIENT_IPPORT = ip + ":" + strconv.Itoa(RPC_client_port)
	println("RPC PORT FOR CLIENTS: " + RPC_CLIENT_IPPORT)

	// UDP PING AND PING RECEIVE
	PingAddr, err := net.ResolveUDPAddr("udp", SEND_PING_IPPORT)
	checkError(err)
	ListenAddr, err := net.ResolveUDPAddr("udp", ip+":0")
	checkError(err)
	ListenConn, err := net.ListenUDP("udp", ListenAddr)
	checkError(err)
	RECEIVE_PING_ADDR = ListenConn.LocalAddr().String()
	println("WE'RE LISTENING ON: ", RECEIVE_PING_ADDR)
	println("we're sending pings on: ", SEND_PING_IPPORT)
	joinStorageServers() // Joining the servers through the LB
	go initPingServers(PingAddr)
	UDPService(ListenConn)
}

// If error is non-nil, print it out and halt.
func checkError(err error) {
	if err != nil {
		log.Fatal(os.Stderr, "Error ", err.Error())
		os.Exit(1)
	}
}

/*
 This method will remove a node from the list of server nodes with the specified UDP_IPPORT
 *****Make sure you lock access to the serverList before callng this method*******
*/
func deleteNodeFromList(udpAddr string) {
	//As every node is unique in its UDP address we can assume deletion after we find that address
	//and return right away
	// Storage might have already deleted the node
	if isNewNode(udpAddr) {
		return
	}

	//initialize variable
	i := serverList

	//if there are no servers, return
	//Shouldn't happen, but just in case
	if i == nil {
		return
	}
	//if i is the one we want to delete, remove it and return
	if i.UDP_IPPORT == udpAddr {
		serverList = (*i).NextServer
		return
	}

	//if i is not the one we want, search until it is found
	j := (*i).NextServer

	for j != nil {
		//if found, delete
		if j.UDP_IPPORT == udpAddr {
			(*i).NextServer = (*j).NextServer
			return
		}

		i = (*i).NextServer
		j = (*i).NextServer
	}

	return
}

/*
* Deletes server with IP:PORT equal to 'a' inside of list if it is found
 */
func deleteServerFromList(udp string) {
	next := serverList
	inner := serverList

	for next != nil {
		if !isNewNode(udp) {
			if (*next).UDP_IPPORT == udp {

				for inner != nil {
					if (*inner).NextServer.UDP_IPPORT == next.UDP_IPPORT {
						(*inner).NextServer = (*next).NextServer
						return
					} else if (*inner).UDP_IPPORT == next.UDP_IPPORT {
						serverList = (*inner).NextServer
						return
					}
					inner = (*inner).NextServer
				}
			}

		} else {
			println("Node not found in list")
		}

		next = (*next).NextServer
	}
}

/*
* cycles through list of connected servers and pings them to make sure theyre still active
 */
func initPingServers(LocalAddr *net.UDPAddr) {
	for {
		serverListMutex.Lock()
		next := serverList
		serverListMutex.Unlock()
		for next != nil {
			ServerAddr, err := net.ResolveUDPAddr("udp", (*next).UDP_IPPORT)
			checkError(err)
			Conn, err := net.DialUDP("udp", LocalAddr, ServerAddr)
			checkError(err)
			dead := pingServer(Conn, 0)

			if dead {
				println("Assume node", (*next).UDP_IPPORT, " is dead!!!! HANDLE THAT SHIT")
				serverListMutex.Lock()
				deleteServerFromList((*next).UDP_IPPORT)
				serverListMutex.Unlock()
			} else {
				println("Node ", (*next).UDP_IPPORT, " is still active.")
			}

			next = (*next).NextServer
		}

		timer1 := time.NewTimer(time.Second * 15)
		<-timer1.C
	}
}

/*
* Writes to the UDP connection for a given server and waits for a reply to make sure server is still active
 */
func pingServer(Conn *net.UDPConn, attempt int) (dead bool) {

	msg := "lbping"
	write_buf := []byte(msg)
	_, err := Conn.Write(write_buf)
	checkError(err)
	read_buf := make([]byte, 1024)
	Conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	for {
		_, _, err := Conn.ReadFromUDP(read_buf)
		if err != nil {
			handlePingReply(Conn, err, attempt)
			dead = true
			break
		} else {
			dead = false
			Conn.Close()
			break
		}
	}

	return
}

/*
* Checks to see if server is replying, if not it attempts to ping again, if tried more than 2 times, it returns true
* to state that the server has died
 */
func handlePingReply(Conn *net.UDPConn, err error, attempt int) {
	if e := err.(net.Error); e.Timeout() {

		if attempt < 1 {
			//try to connect to server again
			println("retrying to connect to server node. Retry attempt: " + strconv.Itoa(attempt))
			pingServer(Conn, attempt+1)

		} else {
			//assume server is dead
			Conn.Close()
		}
	}

	if e, ok := err.(net.Error); !ok || !e.Timeout() {
		// error that isn't a timeout error
		println(e)
		Conn.Close()
	}
}

/*
* Waits for pings, ie Reads from UDP socket
 */
func UDPService(ServerConn *net.UDPConn) {

	buf := make([]byte, 1500)
	for {

		n, addr, err := ServerConn.ReadFromUDP(buf)
		checkError(err)

		go handleUDP(string(buf[0:n]), ServerConn, addr)
	}
}

/*
* write back to server after a ping is received
 */
func handleUDP(recmsg string, Conn *net.UDPConn, addr *net.UDPAddr) {

	buf := []byte(RECEIVE_PING_ADDR)
	_, err := Conn.WriteToUDP(buf, addr)
	checkError(err)
}

/*
* listening for RPC calls from the other servers
 */
func systemListenServe(local string, c chan int) {
	ll, ee := net.Listen("tcp", local)
	nodePORT := ll.Addr().(*net.TCPAddr).Port
	c <- nodePORT
	if ee != nil {
		log.Fatal("listen error:", ee)
	}
	for {
		conn, _ := ll.Accept()
		go rpc.ServeConn(conn)
		Logger.LogLocalEvent("accepted server call")
	}
}

/*
* listening for RPC calls from the clients
 */
func clientListenServe(local string, ch chan int) {
	ll, ee := net.Listen("tcp", local)
	nodePORT := ll.Addr().(*net.TCPAddr).Port
	ch <- nodePORT
	if ee != nil {
		log.Fatal("listen error:", ee)
	}
	for {
		conn, _ := ll.Accept()
		go rpc.ServeConn(conn)
	}
}

/*
*  Join storage servers
 */
func joinStorageServers() {
	Logger.LogLocalEvent("dialing loadbalancer")
	systemService, err := rpc.Dial("tcp", LOAD_BALANCER_IPPORT)
	checkError(err)

	var reply NodeListReply

	newNodeSetup := NewNodeSetup{
		RPC_CLIENT_IPPORT: RPC_CLIENT_IPPORT,
		RPC_SERVER_IPPORT: RPC_SYSTEM_IPPORT,
		UDP_IPPORT:        RECEIVE_PING_ADDR}

	Logger.LogLocalEvent("registering new node")
	err = systemService.Call("NodeService.NewNode", newNodeSetup, &reply)
	checkError(err)

	list := reply.ListOfNodes
	Logger.LogLocalEvent("new node registered, received list of nodes")

	i := list
	println("\nNodes So Far")
	for i != nil {
		println("Node w\\UDP: ", i.UDP_IPPORT)
		i = (*i).NextServer
	}
	println("")

	serverListMutex.Lock()
	serverList = list
	serverListMutex.Unlock()

}

/*
* Add a node to our linked list of server nodes
 */
func addNode(udp string, clientRPC string, serverRPC string) {

	serverListMutex.Lock()
	if RECEIVE_PING_ADDR == udp {
		serverListMutex.Unlock()
		return
	}
	if isNewNode(udp) {

		newNode := &ServerItem{udp, clientRPC, serverRPC, 0, nil}

		if serverList == nil {
			serverList = newNode
		} else {
			newNode.NextServer = serverList
			serverList = newNode
		}
	}
	serverListMutex.Unlock()
	return
}

func isNewNode(ident string) bool {

	next := serverList

	for next != nil {
		if (*next).UDP_IPPORT == ident {
			return false
		}
		next = (*next).NextServer
	}

	return true
}

func sizeOfServerList() (total int) {
	next := serverList
	total = 0
	for next != nil {
		total++
		next = (*next).NextServer
	}

	return
}

/*
* Add A node to our linked list of clients
 */
func addClient(username string, rpc string) {

	clientListMutex.Lock()
	if isNewClient(username) {
		newNode := &ClientItem{username, rpc, nil}

		if clientList == nil {
			clientList = newNode
		} else {
			newNode.NextClient = clientList
			clientList = newNode
		}
	} else {
		next := clientList
		for next != nil {
			if (*next).Username == username {
				(*next).RPC_IPPORT = rpc
				break
			}
			next = (*next).NextClient
		}

	}

	clientListMutex.Unlock()
	return
}

/*
* Checks whether a given username is already in the clientList
 */

func isNewClient(ident string) bool {
	next := clientList

	for next != nil {
		if (*next).Username == ident {
			return false
		}
		next = (*next).NextClient
	}

	return true
}

/*
* Returns the RPC Address of a username if the username if in clientList, else return "not found"
 */
func returnClientAddr(ident string) string {

	next := clientList

	for next != nil {
		if (*next).Username == ident {
			return (*next).RPC_IPPORT
		}
		next = (*next).NextClient
	}

	return "not found"
}

/*
* Returns the size of the clientList
 */
func sizeOfClientList() (total int) {
	next := clientList
	total = 0
	for next != nil {
		total++
		next = (*next).NextClient
	}

	return
}

func sendPublicMsgServers(message ClientMessage) {

	serverListMutex.Lock()
	next := serverList
	size := sizeOfServerList()
	serverListMutex.Unlock()
	var wg sync.WaitGroup
	wg.Add(size)

	clockedMsg := ClockedClientMsg{
		ClientMsg: message,
		ServerId:  RECEIVE_PING_ADDR,
		Clock:     thisClock}

	toHistoryBuf[numMsgsRcvd-1] = clockedMsg

	for next != nil {
		go func(next *ServerItem, clockedMsg ClockedClientMsg) {
			defer wg.Done()
			if (*next).UDP_IPPORT != RECEIVE_PING_ADDR {
				Logger.LogLocalEvent("dialing server")
				systemService, err := rpc.Dial("tcp", (*next).RPC_SERVER_IPPORT)
				//checkError(err)
				if err != nil {
					println("SendPublicMsg To Servers: Server ", (*next).UDP_IPPORT, " isn't accepting tcp conns so skip it...")
					//it's dead but the ping will eventually take care of it
				} else {
					var reply ServerReply
					Logger.LogLocalEvent("broadcasting public message")
					err = systemService.Call("NodeService.SendPublicMsg", clockedMsg, &reply)
					//checkError(err)
					if err == nil {
						fmt.Println("we sent a message to a server: ", reply.Message)
					} else {
						println("SendPublicMsg To Servers: Server ", (*next).UDP_IPPORT, " error on call.")
					}
					systemService.Close()
				}
			}

		}(next, clockedMsg)
		next = (*next).NextServer
	}

	wg.Wait()
	return
}

func sendPublicMsgClients(message ClientMessage) {
	println("inside sendPublicMsgClients: ", message.Message)
	clientListMutex.Lock()
	next := clientList
	size := sizeOfClientList()
	clientListMutex.Unlock()
	var wg sync.WaitGroup
	wg.Add(size)

	for next != nil {
		go func(next *ClientItem, message ClientMessage) {
			defer wg.Done()
			if (*next).Username != message.Username {
				Logger.LogLocalEvent("dialing client")
				systemService, err := rpc.Dial("tcp", (*next).RPC_IPPORT)
				//checkError(err)
				if err != nil {
					println("SendPublicMsg To Clients: Client ", (*next).Username, " isn't accepting tcp conns so skip it... ")
					//DELETE CLIENT IF CONNECTION NO LONGER ACCEPTING
					clientListMutex.Lock()
					deleteClientFromList((*next).Username)
					clientListMutex.Unlock()
				} else {
					var reply ServerReply
					Logger.LogLocalEvent("sending public message")
					// client api uses ClientMessageService
					errr := systemService.Call("ClientMessageService.ReceiveMessage", message, &reply)
					checkError(errr)
					systemService.Close()
				}
			}
		}(next, message)

		next = (*next).NextClient
	}
	wg.Wait()
	return
}

func storeFile(file FileData) {
	path := "../Files/"
	err := os.MkdirAll(path, 0777)
	checkError(err)
	f, er := os.Create(path + file.FileName)
	checkError(er)
	n, error := f.Write(file.Data)
	checkError(error)
	println("bytes written to file: ", n)
	f.Close()
}

func sendPublicFileServers(file FileData) {
	println("inside send public msg servers")
	serverListMutex.Lock()
	next := serverList
	size := sizeOfServerList()
	serverListMutex.Unlock()
	var wg sync.WaitGroup

	wg.Add(size)

	for next != nil {
		go func(next *ServerItem, file FileData) {
			defer wg.Done()
			if (*next).UDP_IPPORT != RECEIVE_PING_ADDR {
				Logger.LogLocalEvent("dialing server")
				systemService, err := rpc.Dial("tcp", (*next).RPC_SERVER_IPPORT)
				//checkError(err)
				if err != nil {
					println("SendPublicMsg To Servers: Server ", (*next).UDP_IPPORT, " isn't accepting tcp conns so skip it...")
					//it's dead but the ping will eventually take care of it
				} else {
					var reply ServerReply
					Logger.LogLocalEvent("transfer public file")
					err = systemService.Call("NodeService.SendPublicFile", file, &reply)
					checkError(err)
					if err == nil {
						fmt.Println("sent file to server: ", reply.Message)
					}
					systemService.Close()
				}
			}
		}(next, file)
		next = (*next).NextServer
	}
	wg.Wait()
	return
}

func sendPublicFileClients(file FileData) {
	println("inside sendPub file clients")
	clientListMutex.Lock()
	next := clientList
	size := sizeOfClientList()
	clientListMutex.Unlock()
	var wg sync.WaitGroup
	wg.Add(size)

	for next != nil {
		go func(next *ClientItem, file FileData) {
			defer wg.Done()
			if (*next).Username != file.Username {
				Logger.LogLocalEvent("dialing client")
				systemService, err := rpc.Dial("tcp", (*next).RPC_IPPORT)
				//checkError(err)
				if err != nil {
					println("SendPublicMsg To Clients: Client ", (*next).Username, " isn't accepting tcp conns so skip it... ")
					//DELETE CLIENT IF CONNECTION NO LONGER ACCEPTING
					clientListMutex.Lock()
					deleteClientFromList((*next).Username)
					clientListMutex.Unlock()
				} else {
					var reply ServerReply
					Logger.LogLocalEvent("transfer public file")
					err = systemService.Call("ClientMessageService.TransferFile", file, &reply)
					checkError(err)
					if err == nil {
						fmt.Println("sent file to client: ", reply.Message)
					}
					systemService.Close()
				}
			}
		}(next, file)

		next = (*next).NextClient
	}

	wg.Wait()
	return
}

//
//This method will remove a node from the list of server nodes with the specified
//UDP_IPPORT
//
//*****Make sure you lock access to the clientList before callng this method*******
func deleteClientFromList(uname string) {

	//initialize variable
	i := clientList

	//if there are no clients, return
	//Shouldn't happen, but just in case
	if i == nil {
		return
	}
	//if i is the one we want to delete, remove it and return
	if i.Username == uname {
		clientList = (*i).NextClient
		return
	}

	//if i is not the one we want, search until it is found
	j := (*i).NextClient

	for j != nil {
		//if found, delete
		if j.Username == uname {
			(*i).NextClient = (*j).NextClient
			return
		}

		i = (*i).NextClient
		j = (*i).NextClient
	}

	return
}

func getIP() (ip string) {

	host, _ := os.Hostname()
	addrs, _ := net.LookupIP(host)
	for _, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil && !ipv4.IsLoopback() {
			ip = ipv4.String()
		}
	}
	return ip
}

func getAddr(uname string) string {
	Logger.LogLocalEvent("dialing loadbalancer")
	systemService, err := rpc.Dial("tcp", LOAD_BALANCER_IPPORT)
	checkError(err)

	var reply ServerReply

	clientRequest := ClientRequest{
		Username: uname}

	Logger.LogLocalEvent("requesting client address")
	err = systemService.Call("NodeService.GetClientAddr", clientRequest, &reply)
	checkError(err)

	fmt.Println("we received a reply from the server: ", reply.Message)
	Logger.LogLocalEvent("received client address")
	systemService.Close()
	return reply.Message
}

func checkBufFull() {
	if numMsgsRcvd == 50 {
		writeHistoryToFile(toHistoryBuf)

		// flush all variables
		thisClock = 0
		numMsgsRcvd = 0
		//toHistoryBuf = nil
	}
}

func writeHistoryToFile(toHistoryBuf []ClockedClientMsg) {

	// this server's chat history filename
	noPeriods := strings.Replace(RECEIVE_PING_ADDR, ".", "", -1)
	safeFile := strings.Replace(noPeriods, ":", "-", 1)

	_, err := os.Stat("../ChatHistory/" + safeFile + ".txt")

	if os.IsNotExist(err) {

		path := "../ChatHistory/"
		err = os.MkdirAll(path, 0777)
		if err != nil {
			println("error: couldn't make chat history folder")
		}
		checkError(err)

		// create chat history file with server ID
		f, er := os.Create("../ChatHistory/" + safeFile + ".txt")
		if er != nil {
			println("error: couldn't create chat history file")
		}
		checkError(er)
		f.Close()
	}

	f, errr := os.OpenFile("../ChatHistory/"+safeFile+".txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	if errr != nil {
		println("error: couldn't open this chat history file")
	}
	defer f.Close()

	for i := 0; i < len(toHistoryBuf); i++ {
		msg := toHistoryBuf[i]
		uname := msg.ClientMsg.Username
		clientmes := msg.ClientMsg.Message
		serverid := msg.ServerId
		clock := msg.Clock
		stringClock := strconv.Itoa(clock)

		n, erro := f.WriteString("{Username: " + uname + ", Message: " + clientmes + ", ServerId: " + serverid + ", clock: " + stringClock + "}\n")
		if erro != nil {
			println("error: couldn't write message to file")
		} else {
			println("we wrote ", n, " bytes")
		}
	}

	return
}

func possessFile(filename string) (reply FileData) {
	path := "../Files/" + filename

	fi, err := os.Stat(path)

	if os.IsNotExist(err) {
		reply = FileData{
			Username: "404"}

	} else {
		// re-open file
		var file, errr = os.OpenFile(path, os.O_RDWR, 0644)
		checkError(errr)
		defer file.Close()

		Data := make([]byte, fi.Size())

		_, _ = file.Read(Data)
		checkError(err)

		reply = FileData{
			Username: "202",
			FileName: filename,
			FileSize: fi.Size(),
			Data:     Data}
	}
	return
}
