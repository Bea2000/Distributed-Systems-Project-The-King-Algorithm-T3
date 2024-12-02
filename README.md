# Distributed Systems Project: The King Algorithm

This repository contains the implementation of **The King Algorithm**, developed during the second semester of 2024 for the **Distributed Systems course** at the Pontificia Universidad Católica de Chile.

## Context

The goal of this project was to implement a fault-tolerant distributed algorithm for solving the Byzantine Generals Problem using the King Algorithm. The solution leverages Go's networking capabilities and employs socket communication to simulate independent processes (nodes) running on different machines. The nodes communicate over a virtual private network (VPN), ensuring distributed execution.

## Requirements

- **Go**: Version 1.20 or later
- **WireGuard VPN**: Set up to allow communication between nodes
- **Two or more machines** connected to the same VPN
- Operating system: Linux, macOS, or Windows

## Setup Instructions

### VPN Configuration

You can use any WireGuard server, but for this project, we used a VPS located in Santiago, with a public IPv4 address.

To install WireGuard on Ubuntu, run the following commands:

```bash
sudo apt update
sudo apt install wireguard
```

#### Generating Keys

To generate your private key, run the following command:

```bash
wg genkey | sudo tee /etc/wireguard/private.key
sudo chmod go= /etc/wireguard/private.key
```

To generate your public key, run the following command:

```bash
sudo cat /etc/wireguard/private.key | wg pubkey | sudo tee /etc/wireguard/public.key
```

The output of this command will be your public key.

#### Configuration

Once you have the following details:

- Server's public IP
- Client's IP address
- Server's public key
- Client's public key
- Server's IP range

You can create the WireGuard configuration file. To do so, create the file /etc/wireguard/wg0.conf with the command:

```bash
sudo nano /etc/wireguard/wg0.conf
```

And paste the following configuration:

```bash
[Interface]
PrivateKey = <base64_encoded_peer_private_key>
Address = <client_ip_address>

[Peer]
PublicKey = <base64_encoded_server_public_key>
AllowedIPs = <full_address_range>
Endpoint = <server_public_ip_addr>:51820
```

#### Starting the VPN

To initialize the tunnel, run the following command:

```bash
sudo wg-quick up wg0
```

You can check the status of the tunnel with the command:

```bash
sudo wg
```

You should see something like the following:

```bash
interface: wg0
  public key: <base64_encoded_peer_public_key>
  private key: (hidden)
  listening port: 49338
  fwmark: 0xca6c
peer: <base64_encoded_server_public_key>
  endpoint: <server_public_ip_addr>:51820
  allowed ips: <full_address_range>
  latest handshake: 1 second ago
  transfer: 6.50 KiB received, 15.41 KiB sent
```

### Clone the Repository

```bash
git clone https://github.com/your-username/repository.git
cd repository
```

### Build the Project

```bash
go build -o king main.go
```

### Run the Project

#### Starting the Nodes

```bash
./king -adressesList=<IP1,IP2,...> -nodes=<NUM_NODES> -nodeIds=<ID1,ID2,...>
```

- `-adressesList` is a comma-separated list of the private IP addresses of the nodes. By default, it's set to `127.0.0.1`, which means all nodes will be run on this machine.
- `-nodes` is the total number of nodes in the system. By default, it's set to `5`.
- `-nodeIds` is a comma-separated list of the IDs of the nodes to run on this machine.

**Note:** If there's only one address, all nodes will be run on this machine. If there's more than one address, only the specified nodes will be run on this machine.

#### Watching the nodes

The program outputs logs detailing the node interactions, selected king per round, and final consensus.

### Example Output

```plaintext
Node 4 received all ready messages
Node 1 received all ready messages
Node 2 received all ready messages
Node 3 received all ready messages
Node 1 chose king 2 for round 1
Node 2 chose king 2 for round 1
Node 1 sent plan to 2 on round 1
Node 2 received plan from 1 on round 1
Node 2 chose king 2 for round 1
Node 3 sent plan to 2 on round 1
...
Final consensus achieved: Attack
```
