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
	highestBid  int32
	auctionOver bool
	isLeader    bool
	nodeID      int32
	leaderID    int32
	peers       []pb.AuctionClient
	mutex       sync.Mutex
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
	if req.NodeId > s.nodeID {
		s.mutex.Lock()
		s.isLeader = false
		s.mutex.Unlock()
		go s.startElection()
	}
	return &pb.ElectionResponse{}, nil
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

	for _, peer := range s.peers {
		_, err := peer.Election(context.Background(), &pb.ElectionRequest{NodeId: s.nodeID})
		if err == nil {
			return
		}
	}

	s.mutex.Lock()
	s.leaderID = s.nodeID
	s.isLeader = true
	s.mutex.Unlock()

	for _, peer := range s.peers {
		_, _ = peer.Victory(context.Background(), &pb.VictoryRequest{LeaderId: s.nodeID})
	}
}

func (s *auctionServer) startAuction(duration time.Duration) {
	time.Sleep(duration)
	s.mutex.Lock()
	s.auctionOver = true
	s.mutex.Unlock()
}

func main() {
	var (
		portID      = flag.Int("portID", 0, "Port ID")
		baseport    = flag.Int("basePort", 0, "Base Port")
		servercount = flag.Int("serverCount", 0, "Server Count")
		isLeader    = flag.Bool("isLeader", false, "Is Leader")
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

	server.isLeader = *isLeader
	server.leaderID = int32(*baseport)

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
			log.Printf("failed to connect to peer %s: %v", addr, err)
		}
	}

	// Start the auction with a 2-minute timer
	go server.startAuction(2 * time.Minute)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// go run server.go --portid=? --baseport=? --servercount=? --isleader=true
