package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	pb "auctionService/proto/github.com/BirdyDK/DS-handin5"

	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewAuctionClient(conn)
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Auction Client: Enter 'bid <amount>' to place a bid or 'result' to check the highest bid.")

	for scanner.Scan() {
		input := scanner.Text()
		parts := strings.Split(input, " ")
		command := parts[0]

		switch command {
		case "bid":
			if len(parts) != 2 {
				fmt.Println("Usage: bid <amount>")
				continue
			}
			amount, err := strconv.Atoi(parts[1])
			if err != nil {
				fmt.Println("Invalid amount. Please enter an integer.")
				continue
			}

			bidResponse, err := client.Bid(context.Background(), &pb.BidRequest{Amount: int32(amount)})
			if err != nil {
				log.Fatalf("could not bid: %v", err)
			}
			fmt.Println(bidResponse.Status)

		case "result":
			resultResponse, err := client.Result(context.Background(), &pb.ResultRequest{})
			if err != nil {
				log.Fatalf("could not get result: %v", err)
			}
			fmt.Printf("Highest bid: %d, Status: %s\n", resultResponse.HighestBid, resultResponse.Winner)

		default:
			fmt.Println("Unknown command. Enter 'bid <amount>' or 'result'.")
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
