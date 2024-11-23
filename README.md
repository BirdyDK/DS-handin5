# DS-handin5

The program takes 4 parameters:

- PortID (--portID): The port for the current server.
- BasePort (--basePort): This will be the leader and the basis for the other server based on the server count.
- ServerCount (--serverCount): The amount of servers.

The basePort should always be set to 50051 and one of the portID's should be 50051 since the clients make calls on port 50051.

**Example of how to run the program:**

go run .\server\server.go --portID=50051 --basePort=50051 --serverCount=4

go run .\server\server.go --portID=50052 --basePort=50051 --serverCount=4

go run .\server\server.go --portID=50053 --basePort=50051 --serverCount=4

go run .\server\server.go --portID=50054 --basePort=50051 --serverCount=4

For each client you will run the the following for as many clients as you want in the auction (each needs their own terminal):

go run .\client\client.go

