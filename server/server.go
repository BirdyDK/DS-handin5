package main

import (
	"context"
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

func (s *auctionServer) startAuction(duration time.Duration) {
	time.Sleep(duration)
	s.mutex.Lock()
	s.auctionOver = true
	s.mutex.Unlock()
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	server := &auctionServer{}
	pb.RegisterAuctionServer(grpcServer, server)

	// Start the auction with a 2-minute timer
	go server.startAuction(2 * time.Minute)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
