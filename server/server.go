package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	pb "auctionService/proto/github.com/BirdyDK/DS-handin5"

	"google.golang.org/grpc"
)

type auctionServer struct {
	pb.UnimplementedAuctionServer
	highestBid             int32
	auctionOver            bool
	isLeader               bool
	nodeID                 int32
	baseport               int32
	serverCount            int32
	leaderID               int32
	peers                  []pb.AuctionClient
	mutex                  sync.Mutex
	electionInProgress     bool
	higherServer           bool
	electionCountdown      int
	resetElectionCountdown chan bool
	Timer                  time.Timer
}

func (s *auctionServer) Bid(ctx context.Context, req *pb.BidRequest) (*pb.BidResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.auctionOver {
		return &pb.BidResponse{Status: "exception: auction is over"}, nil
	}

	if req.Amount > s.highestBid {
		s.highestBid = req.Amount
		return &pb.BidResponse{Status: fmt.Sprintf("success: you're now the highest bidder with %d", req.Amount)}, nil
	}
	return &pb.BidResponse{Status: "fail: your bid was too low"}, nil
}

func (s *auctionServer) Result(ctx context.Context, req *pb.ResultRequest) (*pb.ResultResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.auctionOver {
		return &pb.ResultResponse{HighestBid: s.highestBid, Winner: "auction is still ongoing"}, nil
	}

	return &pb.ResultResponse{HighestBid: s.highestBid, Winner: "auction is over, final result"}, nil
}

func (s *auctionServer) Election(ctx context.Context, req *pb.ElectionRequest) (*pb.ElectionResponse, error) {
	/*if req.NodeId < s.nodeID {
		s.mutex.Lock()
		s.isLeader = false
		s.mutex.Unlock()
		go s.startElection()
	}*/
	if !s.electionInProgress {
		s.electionInProgress = true
		s.higherServer = req.NodeId > s.nodeID
		s.electionCountdown = 20
		s.resetElectionCountdown = make(chan bool, 100)
		go s.RunningElection()
	}

	if req.NodeId > s.nodeID {
		s.higherServer = true
	} else {
		s.startElection()
	}
	s.resetElectionCountdown <- true

	return &pb.ElectionResponse{}, nil
}

func (s *auctionServer) RunningElection() {
	for {
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
			s.electionCountdown = 20
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *auctionServer) BecomeLeader() {

}

func (s *auctionServer) Victory(ctx context.Context, req *pb.VictoryRequest) (*pb.VictoryResponse, error) {
	s.mutex.Lock()
	s.leaderID = req.LeaderId
	s.isLeader = (s.leaderID == s.nodeID)
	s.mutex.Unlock()
	return &pb.VictoryResponse{}, nil
}

func (s *auctionServer) startElection() {
	s.mutex.Lock()
	s.isLeader = false
	s.mutex.Unlock()
	higherIDNodes := false

	for id, peer := range s.peers {
		if id > int(s.nodeID) {
			higherIDNodes = true
			_, err := peer.Election(context.Background(), &pb.ElectionRequest{NodeId: s.nodeID})
			if err == nil {
				return
			}
		}
	}

	if !higherIDNodes {
		s.mutex.Lock()
		s.leaderID = s.nodeID
		s.isLeader = true
		s.mutex.Unlock()
		for _, peer := range s.peers {
			_, _ = peer.Victory(context.Background(), &pb.VictoryRequest{LeaderId: s.nodeID})
		}
	}
}

func (s *auctionServer) startAuction(duration time.Duration) {
	time.Sleep(duration)
	s.mutex.Lock()
	s.auctionOver = true
	s.mutex.Unlock()
}
func (s *auctionServer) PingSubordinates() {
	for {
		for _, peer := range s.peers {
			_, _ = peer.LeaderMessage(context.Background(), &pb.LeaderMessageRequest{Message: "the leader is trying to reach you"})
		}
		time.Sleep(5 * time.Second)
	}
}
func (s *auctionServer) LeaderMessage(ctx context.Context, req *pb.LeaderMessageRequest) (*pb.LeaderMessageResponse, error) {
	log.Println(req)
	s.TimerReset()
	return &pb.LeaderMessageResponse{}, nil
}

func (s *auctionServer) TimerReset() {
	s.Timer.Reset(11 * time.Second)
}

func (s *auctionServer) TimerCheck() {
	for !s.isLeader {
		if len(s.Timer.C) > 0 {
			<-s.Timer.C
			s.startElection()

		}
		time.Sleep(time.Second)
	}
	if s.isLeader {
		s.Timer.Stop()
	}
}

func main() {
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

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *portID))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	server := &auctionServer{}
	pb.RegisterAuctionServer(grpcServer, server)

	server.isLeader = *portID == *baseport
	server.leaderID = int32(*baseport)
	server.baseport = int32(*baseport)
	server.nodeID = int32(*portID)
	server.serverCount = int32(*servercount)

	var peerAddrList []int
	for i := *baseport; i < *servercount; i++ {
		if i == *portID {
			continue
		}

		peerAddrList = append(peerAddrList, i)
	}

	for _, addr := range peerAddrList {
		conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", addr), grpc.WithInsecure())
		if err == nil {
			server.peers = append(server.peers, pb.NewAuctionClient(conn))
		} else {
			log.Printf("failed to connect to peer %d: %v", addr, err)
		}
	}
	if server.baseport == server.nodeID {
		go server.PingSubordinates()
	}

	// Start the auction with a 2-minute timer
	if server.isLeader {
		go server.startAuction(2 * time.Minute)
	}

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
	server.Timer = *time.NewTimer(20 * time.Second)
	go server.TimerCheck()

}

// go run server.go --portid=? --baseport=? --servercount=? --isleader=true
