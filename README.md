# DS-handin5

The program takes 4 parameters:

- PortID (--portID): The port for the current server.
- BasePort (--basePort): This will be the leader and the basis for the other server based on the server count.
- ServerCount (--serverCount): The amount of servers.
- IsLeader (--isLeader): This is only required for the one where PortID is the same a BasePort to say Im the leader.

**Example of how to run the program:**

go run server.go --portid=5000 --baseport=5000 --servercount=3 --isleader=true

go run server.go --portid=5001 --baseport=5000 --servercount=3

go run server.go --portid=5002 --baseport=5000 --servercount=3

For each client you will run the the following for as many clients as you want in the auction:

?
