package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"slices"
	"sync"
	"time"

	pb "auctionService/proto/github.com/BirdyDK/DS-handin5"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type auctionServer struct {
	pb.UnimplementedAuctionServer
	highestBid             int32
	highestBidder          string
	registeredUsers        []string
	auctionTimer           int32
	auctionOver            bool
	isLeader               bool
	nodeID                 int32
	baseport               int32
	serverCount            int32
	leaderID               int32
	peers                  []pb.AuctionClient
	peerAddrList           []int32
	mutex                  sync.Mutex
	electionInProgress     bool
	higherServer           bool
	electionCountdown      int
	resetElectionCountdown chan bool
	durationTimer          int
	Timer                  time.Timer
	serverPort             int32
}

func (s *auctionServer) Bid(ctx context.Context, req *pb.BidRequest) (*pb.BidResponse, error) {
	log.Println("Bid")
	s.mutex.Lock()
	defer s.mutex.Unlock()

	p, _ := peer.FromContext(ctx)
	ps := p.Addr.String()
	if !slices.Contains(s.registeredUsers, ps) {
		s.registeredUsers = append(s.registeredUsers, ps)
	}

	if s.auctionOver {
		return &pb.BidResponse{Status: "exception: auction is over"}, nil
	}

	if req.Amount > s.highestBid {
		s.highestBid = req.Amount
		s.highestBidder = ps
		for _, peer := range s.peers {
			_, _ = peer.Status(context.Background(), &pb.StatusRequest{HighestBid: s.highestBid, HighestBidder: s.highestBidder, RegisteredUsers: s.registeredUsers, Time: s.auctionTimer})
		}
		return &pb.BidResponse{Status: fmt.Sprintf("success: you're now the highest bidder with %d", req.Amount)}, nil
	}
	for _, peer := range s.peers {
		_, _ = peer.Status(context.Background(), &pb.StatusRequest{HighestBid: s.highestBid, HighestBidder: s.highestBidder, RegisteredUsers: s.registeredUsers, Time: s.auctionTimer})
	}
	return &pb.BidResponse{Status: "fail: your bid was too low"}, nil
}

func (s *auctionServer) Result(ctx context.Context, req *pb.ResultRequest) (*pb.ResultResponse, error) {
	log.Println("Result")
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.auctionOver {
		return &pb.ResultResponse{HighestBid: s.highestBid, Winner: ("auction is still ongoing." + s.highestBidder + " is leading")}, nil
	}

	return &pb.ResultResponse{HighestBid: s.highestBid, Winner: ("auction is over, final result, " + s.highestBidder + " has won")}, nil
}

func (s *auctionServer) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	log.Println("Status")
	s.highestBid = req.HighestBid
	s.highestBidder = req.HighestBidder
	s.registeredUsers = req.RegisteredUsers
	s.auctionTimer = req.Time
	return &pb.StatusResponse{}, nil
}

func (s *auctionServer) Election(ctx context.Context, req *pb.ElectionRequest) (*pb.ElectionResponse, error) {
	log.Println("Election")

	if !s.electionInProgress {
		s.electionInProgress = true
		s.higherServer = req.NodeId > s.nodeID
		s.electionCountdown = 10
		s.resetElectionCountdown = make(chan bool, 100)
		go s.RunningElection()
	}

	log.Println(req.NodeId, " v. ", s.nodeID)
	if req.NodeId > s.nodeID {
		s.higherServer = true
	} else {
		s.startElection()
	}
	s.resetElectionCountdown <- true

	return &pb.ElectionResponse{}, nil
}

func (s *auctionServer) RunningElection() {
	log.Println("RunningElection")
	for {
		// log.Println("Election time ", s.electionCountdown)
		if s.electionCountdown == 0 {
			s.electionInProgress = false
			if !s.higherServer {
				s.BecomeLeader()
			}
			break
		}
		s.electionCountdown--
		if len(s.resetElectionCountdown) > 0 {
			<-s.resetElectionCountdown
			s.electionCountdown = 10
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *auctionServer) BecomeLeader() {
	log.Println("BecomeLeader")
	s.mutex.Lock()
	s.serverPort = s.baseport
	s.leaderID = s.nodeID
	s.isLeader = (s.leaderID == s.nodeID)
	s.mutex.Unlock()

	go s.PingSubordinates()

	// Rebind to port 5000 as leader
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.serverPort))
	if err != nil {
		log.Fatalf("failed to listen on port 5000: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterAuctionServer(grpcServer, s)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
}

func (s *auctionServer) startElection() {
	log.Println("StartElection")
	s.durationTimer = -1
	s.mutex.Lock()
	s.isLeader = false
	s.mutex.Unlock()

	if len(s.peers) == 0 {
		s.BecomeLeader()
		return
	}

	for index, peer := range s.peers {
		log.Println("Index: ", index, " Peer: ", peer, " ID: ", s.peerAddrList[index])
		_, err := peer.Election(context.Background(), &pb.ElectionRequest{NodeId: s.nodeID})
		if err != nil {
			return
		}
	}

	/*
		if !higherIDNodes {
			s.mutex.Lock()
			s.leaderID = s.nodeID
			s.isLeader = true
			s.mutex.Unlock()
			for _, peer := range s.peers {
				_, _ = peer.Victory(context.Background(), &pb.VictoryRequest{LeaderId: s.nodeID})
			}
		}
	*/
}

func (s *auctionServer) startAuction(durationSec int32) {
	log.Println("StartAuction")
	s.auctionTimer = durationSec
	for s.auctionTimer > 0 {
		s.auctionTimer--
		time.Sleep(time.Second)
	}
	s.mutex.Lock()
	s.auctionOver = true
	s.mutex.Unlock()
	s.auctionCooldown()
}

func (s *auctionServer) auctionCooldown() {
	log.Println("AuctionCooldown")
	for s.auctionTimer > -60 {
		s.auctionTimer--
		time.Sleep(time.Second)
	}
	s.highestBid = 0
	s.highestBidder = ""
	s.mutex.Lock()
	s.auctionOver = false
	s.mutex.Unlock()
	s.startAuction(120)
}

func (s *auctionServer) PingSubordinates() {
	for {
		log.Println("PingSubordinates")
		//fmt.Println("pinging subordinates")
		for _, peer := range s.peers {
			//fmt.Println("pinging subordinates for real this time")
			_, _ = peer.LeaderMessage(context.Background(), &pb.LeaderMessageRequest{Message: "the leader is trying to reach you"})
		}
		time.Sleep(1 * time.Second)
	}
}
func (s *auctionServer) LeaderMessage(ctx context.Context, req *pb.LeaderMessageRequest) (*pb.LeaderMessageResponse, error) {
	log.Println("LeaderMessage")
	//fmt.Println("ping message")
	//log.Println(req)
	//fmt.Println("ping message over")
	s.TimerReset()
	return &pb.LeaderMessageResponse{}, nil
}

func (s *auctionServer) TimerReset() {
	log.Println("TimerReset")
	s.durationTimer = 3
	//log.Println("timer is reset")
}

func (s *auctionServer) TimerCheck() {
	for !s.isLeader {
		if s.durationTimer < 0 {
			continue
		}
		log.Println("TimerCheck")
		s.durationTimer--
		if s.durationTimer == 0 {
			log.Println("Timer Ran Out")
			s.startElection()

		}
		time.Sleep(time.Second)
	}
	if s.isLeader {
		s.durationTimer = -1
	}
}

func main() {
	log.Println("Main")
	var (
		portID      = flag.Int("portID", 0, "Port ID")
		baseport    = flag.Int("basePort", 0, "Base Port")
		servercount = flag.Int("serverCount", 0, "Server Count")
	)
	flag.Parse()

	if *portID == 0 {
		log.Println("Port ID must be specified")
	}

	if *baseport == 0 {
		log.Println("Base Port must be specified")
	}

	if *servercount == 0 {
		log.Println("Server count must be specified")
	}

	initialPort := *portID
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", initialPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	server := &auctionServer{serverPort: int32(initialPort)}
	pb.RegisterAuctionServer(grpcServer, server)

	server.isLeader = *portID == *baseport
	server.leaderID = int32(*baseport)
	server.baseport = int32(*baseport)
	server.nodeID = int32(*portID)
	server.serverCount = int32(*servercount)
	server.durationTimer = 60

	for i := *baseport; i < *baseport+*servercount; i++ {
		if i == *portID || i == *baseport {
			continue
		}

		server.peerAddrList = append(server.peerAddrList, int32(i))
	}
	fmt.Println(server.peerAddrList)
	for _, addr := range server.peerAddrList {
		conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", addr), grpc.WithInsecure())
		if err == nil {
			server.peers = append(server.peers, pb.NewAuctionClient(conn))
		} else {
			log.Printf("failed to connect to peer %d: %v", addr, err)
		}
	}
	fmt.Println(server.peers)
	if server.baseport == server.nodeID {
		go server.PingSubordinates()
	}

	// Start the auction with a 2-minute timer

	go server.startAuction(120)

	go server.TimerCheck()
	server.Timer = *time.NewTimer(20 * time.Second)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

}

// go run server.go --portid=? --baseport=? --servercount=? --isleader=true
