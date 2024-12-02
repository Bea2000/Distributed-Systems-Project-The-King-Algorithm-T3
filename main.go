package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
)

type Node struct {
	Id                 int
	Address            string
	Port               string
	KingId             int
	isTraitor          bool
	plan               Plan
	round              int
	plans              map[int][]Plan
	mu                 sync.Mutex
	ln                 net.Listener
	isRunning          bool
	nodeAddressesPorts []NodeAddressPort
	traitors           []int
	ACKs               []int
}

type MessageType string

const (
	PlanMessage MessageType = "plan"
	ACKMessage  MessageType = "ack"
)

type NodeAddressPort struct {
	Id      int
	Address string
	Port    string
}

type Plan struct {
	From int
	Plan string
}

type Message struct {
	Type  MessageType
	From  int
	Round int
	Plan  string
}

func (node *Node) startServer(wg *sync.WaitGroup) {
	// The node listens in an infinite loop
	defer wg.Done()

	ln, err := net.Listen("tcp", node.Address+":"+node.Port)
	if err != nil {
		fmt.Printf("Node %d failed to start server: %v\n", node.Id, err)
		return
	}

	node.ln = ln

	// While the node is running, accept incoming connections
	for node.isRunning {
		conn, err := node.ln.Accept()
		if err != nil {
			continue
		}

		go node.handleConnection(conn)
	}

}

func (node *Node) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Decode the incoming message
	var msg Message
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&msg); err != nil {
		fmt.Printf("Node %d error decoding message: %v\n", node.Id, err)
		return
	}

	// Lock access to the node's data
	node.mu.Lock()
	defer node.mu.Unlock()

	switch msg.Type {
	case ACKMessage:
		if !containsInt(node.ACKs, msg.From) {
			node.ACKs = append(node.ACKs, msg.From)
		}
	case PlanMessage:
		fmt.Printf("Node %d received plan from %d on round %d\n", node.Id, msg.From, msg.Round)
		node.plans[msg.Round] = append(node.plans[msg.Round], Plan{
			From: msg.From,
			Plan: msg.Plan,
		})
	}
}

func (node *Node) sendReadyMessage(target NodeAddressPort) {
	msg := Message{
		Type: ACKMessage,
		From: node.Id,
	}
	node.sendMessage(target, msg)
}

func (node *Node) waitForAllNodes(totalNodes int) {
	expectedAcks := totalNodes - 1 // todos menos uno mismo

	// Enviar READY a todos los otros nodos
	for _, target := range node.nodeAddressesPorts {
		if target.Id != node.Id {
			node.sendReadyMessage(target)
		}
	}

	// Esperar hasta recibir todos los ACKs
	for {
		node.mu.Lock()
		receivedAcks := len(node.ACKs)
		node.mu.Unlock()

		if receivedAcks >= expectedAcks {
			fmt.Printf("Node %d received all ready messages\n", node.Id)
			return
		}
	}
}

func (node *Node) stopServer() {
	// Close the node's listener
	node.ln.Close()
}

func (node *Node) sendMessage(target NodeAddressPort, msg Message) {
	// Establish a TCP connection with the target
	conn, err := net.Dial("tcp", target.Address+":"+target.Port)
	if err != nil {
		fmt.Printf("Node %d failed to send message to %d: %v\n", node.Id, target.Id, err)
		return
	}

	// Encode the message in JSON format and send it over the connection
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(&msg); err != nil {
		fmt.Printf("Node %d error sending message to %d: %v\n", node.Id, target.Id, err)
	}

	if msg.Type == PlanMessage {
		fmt.Printf("Node %d sent plan to %d on round %d\n", node.Id, target.Id, msg.Round)
	}
}

func (node *Node) chooseKing(totalNodes int, round int) int {
	kingId := round % totalNodes

	// If both the current king and the new king are traitors, choose another king
	if contains(node.traitors, node.KingId) && contains(node.traitors, kingId) {
		return node.chooseKing(totalNodes, round+1)
	}

	return kingId
}

func (node *Node) runNode(wg *sync.WaitGroup, rounds int, totalNodes int) {
	defer wg.Done()

	for round := 1; round <= rounds; round++ {
		node.mu.Lock()
		node.round = round
		node.mu.Unlock()

		// Choose the king for the current round
		node.KingId = node.chooseKing(totalNodes, round)
		fmt.Printf("Node %d chose king %d for round %d\n", node.Id, node.KingId, round)

		// Send the plan to the rest of the nodes
		node.broadcastPlan(round)

		// Wait for all nodes to receive the plan
		node.waitForPlans(round, totalNodes)

		fmt.Printf("on round %d, node %d has %d plans: ", round, node.Id, len(node.plans[round]))
		for _, plan := range node.plans[round] {
			fmt.Printf("Node%d plan: %s ", plan.From, plan.Plan)
		}
		fmt.Println()

		// Calculate the majority plan
		majority, majorityPlan := node.calculateMajorityPlan(round)

		if majority {
			// If there's a majority, the node's plan is the majority plan
			node.plan.Plan = majorityPlan
		} else {
			for _, plan := range node.plans[round] {
				if plan.From == node.KingId {
					node.plan.Plan = plan.Plan
				}
			}
		}

	}

	node.mu.Lock()
	node.isRunning = false
	node.stopServer()
	node.mu.Unlock()
}

func (node *Node) broadcastPlan(round int) {
	planToSend := node.plan.Plan

	// Send the plan to all nodes except itself
	for _, target := range node.nodeAddressesPorts {
		if target.Id != node.Id {
			if node.isTraitor {
				// If the node is a traitor, send a random plan
				planToSend = getRandomPlan()
			}
			node.sendMessage(target, Message{
				Type:  PlanMessage,
				From:  node.Id,
				Round: round,
				Plan:  planToSend,
			})
		}
	}
}

func (node *Node) waitForPlans(round int, totalNodes int) {
	for {
		node.mu.Lock()
		plansCount := len(node.plans[round])
		node.mu.Unlock()

		if plansCount == totalNodes-1 {
			break
		}
	}
}

func (node *Node) calculateMajorityPlan(round int) (bool, string) {
	plans := make(map[string]int)

	// Calculate the number of traitors
	numTraitors := int(math.Round(float64(len(node.nodeAddressesPorts)-1) / 4))

	// Count votes for each plan
	for _, msg := range node.plans[round] {
		plans[msg.Plan]++
	}

	// Find the plan with the most votes
	maxVotes := 0
	majorityPlan := ""
	for plan, votes := range plans {
		if votes > maxVotes {
			maxVotes = votes
			majorityPlan = plan
		}
	}

	// Verify if it's overwhelming (more than totalNodes + numTraitors)
	if maxVotes > (len(node.nodeAddressesPorts)/2)+numTraitors {
		node.plan.Plan = majorityPlan
		return true, majorityPlan
	}

	return false, ""
}

func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func getRandomPlan() string {
	plans := []string{"Attack", "Retreat"}
	return plans[rand.Intn(len(plans))]
}

func generateNodes(totalNodes int, addresses []string) ([]*Node, int) {
	nodes := []*Node{}

	traitors := []int{}

	maxTraitors := int(math.Round(float64(totalNodes-1) / 4))
	rounds := maxTraitors + 1

	for i := totalNodes - maxTraitors; i < totalNodes; i++ {
		traitors = append(traitors, i)
	}

	nodeAddressesPorts := []NodeAddressPort{}
	for i := 0; i < totalNodes; i++ {
		nodeAddressesPorts = append(nodeAddressesPorts, NodeAddressPort{
			Id:      i,
			Address: addresses[i%len(addresses)],
			Port:    fmt.Sprintf("800%d", i+1),
		})
	}

	for i := 0; i < totalNodes; i++ {
		nodes = append(nodes, &Node{
			Id:                 i,
			Address:            addresses[i%len(addresses)],
			Port:               fmt.Sprintf("800%d", i+1),
			KingId:             0,
			isTraitor:          contains(traitors, i),
			plan:               Plan{Plan: getRandomPlan()},
			round:              1,
			plans:              make(map[int][]Plan),
			mu:                 sync.Mutex{},
			isRunning:          true,
			ln:                 nil,
			nodeAddressesPorts: nodeAddressesPorts,
			traitors:           traitors,
			ACKs:               []int{},
		})
	}

	return nodes, rounds
}

func getNodeIds(nodeIdsFlag string) []int {
	nodeIdsStr := strings.Split(nodeIdsFlag, ",")
	nodeIds := []int{}
	for _, idStr := range nodeIdsStr {
		id, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			fmt.Printf("Invalid node ID: %s\n", idStr)
			return []int{}
		}
		nodeIds = append(nodeIds, id)
	}
	return nodeIds
}

func main() {
	// Parse the addresses list
	adressesListFlag := flag.String("adressesList", "127.0.0.1", "Comma-separated list of addresses to run on this machine")

	// Parse the number of nodes
	nodesFlag := flag.Int("nodes", 5, "Number of nodes to run on this machine")

	// Parse the node IDs
	nodeIdsFlag := flag.String("nodeIds", "", "Comma-separated list of node IDs to run on this machine")

	flag.Parse()

	addresses := strings.Split(*adressesListFlag, ",")
	totalNodes := *nodesFlag

	nodes, rounds := generateNodes(totalNodes, addresses)

	nodesToRun := []*Node{}
	// If there's only one address, run all nodes
	if len(addresses) == 1 {
		nodesToRun = nodes
	} else {
		// If there's more than one address, run only the specified nodes
		if *nodeIdsFlag == "" {
			fmt.Println("Please specify the node IDs to run on this machine using the -nodeIds flag.")
			return
		}
		nodeIdsToRun := getNodeIds(*nodeIdsFlag)
		for _, node := range nodes {
			if containsInt(nodeIdsToRun, node.Id) {
				nodesToRun = append(nodesToRun, node)
			}
		}
	}

	var wg sync.WaitGroup
	var waitAllNodes sync.WaitGroup

	// Start servers for specified nodes
	for _, node := range nodesToRun {
		wg.Add(1)
		go node.startServer(&wg)
	}

	// Wait for other nodes to be ready
	for _, node := range nodesToRun {
		waitAllNodes.Add(1)
		go func(node *Node) {
			defer waitAllNodes.Done()
			node.waitForAllNodes(totalNodes)
		}(node)
	}

	// Wait for all nodes to be ready
	waitAllNodes.Wait()

	// Run specified nodes
	for _, node := range nodesToRun {
		wg.Add(1)
		go node.runNode(&wg, rounds, totalNodes)
	}

	wg.Wait()

	fmt.Println("Consensus achieved. Final plans:")
	for _, n := range nodesToRun {
		fmt.Printf("Node %d plan: %s\n", n.Id, n.plan.Plan)
	}
}
