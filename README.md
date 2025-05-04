# Go Blockchain Simulator

## Overview
This project is a discrete event simulator for a simplified Proof-of-Work (PoW) style blockchain network, developed in Go. It serves as an educational tool to explore the dynamics and performance characteristics of blockchain systems by allowing users to configure various network and blockchain parameters and observe their impact on metrics like throughput, block intervals, and latency.

## Features

* **Discrete Event Simulation (DES):** Built using Go's standard library (`container/heap`) to manage time-stamped events efficiently.
* **Configurable Parameters:** Most key parameters can be adjusted via command-line flags:
    * Network: Number of Nodes, Number of Miners, Network Delay (Min/Max).
    * Blockchain: Block Size Limit (bytes), Confirmation Depth.
    * Transactions: Injection Rate (TPS), Total Transaction Count, Transaction Size (Normal distribution defined by Min/Max clamps, Mean, Standard Deviation).
    * Mining: Block Finding Time Range (Uniform distribution between Min/Max).
    * Simulation: Total Duration.
* **Simplified PoW Mining:** Block finding time is simulated using a configurable uniform random distribution (no actual hashing or dynamic difficulty adjustment).
* **Miner "Wait for Fullness" Rule:** Miners wait until their mempool reaches 95% byte capacity before attempting to mine a block.
* **Direct Broadcast Network:** Simplified network model where transactions and blocks are broadcast directly to all other nodes with an individual random delay.
* **Basic Fork Resolution:** Nodes switch to the chain with the highest cumulative work (represented by height).
* **Mempool Management:** Nodes maintain local mempools.
* **Statistics and Analysis:** Calculates and reports various metrics at the end of the simulation:
    * Overall Confirmed Throughput (TPS)
    * Average Actual Block Interval
    * Average Block Throughput (relative to a target 10min interval)
    * Average Confirmation Latency
    * Global Stale Block Count
    * Individual Node Statistics
    * Prints the final main chain structure.

## Setup and Installation
1.  **Ensure Go is installed:** You need a working Go environment (Go 1.16+ recommended).
2.  **Get the Code:** Clone or download the source code files (`.go` files, `Makefile`) into a single directory.
3.  **Build:** Open a terminal in the project directory and run:
    ```bash
    make build
    # OR directly:
    # go build -o blockchain-sim *.go
    ```
    This will create an executable file named `blockchain-sim` (or `blockchain-sim.exe` on Windows).

## Configuration
The simulation is configured primarily through command-line flags. Run `./blockchain-sim -h` to see all available flags and their default values.
**Key Flags:**
* `-nodes`: Total number of nodes (e.g., `20`).
* `-miners`: Number of nodes that are miners (e.g., `5`).
* `-duration`: Maximum simulation time (e.g., `1h`, `30m`, `1800s`).
* `-block_size_bytes`: Maximum block size limit in bytes (e.g., `1048576` for 1 MiB).
* `-tx_rate`: Target transaction injection rate (transactions per second, e.g., `10.0`).
* `-total_txs`: Target total number of transactions to inject (e.g., `50000`).
* `-tx_size_mean`: Mean transaction size in bytes for Normal distribution (e.g., `300`).
* `-tx_size_stddev`: Standard deviation for transaction size (e.g., `150`).
* `-tx_size_min`: Minimum transaction size clamp (bytes, e.g., `150`).
* `-tx_size_max`: Maximum transaction size clamp (bytes, e.g., `800`).
* `-find_time_min`: Minimum time to find a block (e.g., `9m`).
* ` -find_time_max`: Maximum time to find a block (e.g., `11m`).
* `-delay_min`: Minimum network broadcast delay (e.g., `100ms`).
* `-delay_max`: Maximum network broadcast delay (e.g., `500ms`).
* `-confirm_depth`: Required block depth for confirmation (e.g., `6`).

## Running the Simulation
Execute the compiled binary with desired flags:
```bash
# Run with default settings
./blockchain-sim

# Run for 2 hours with 50 nodes (12 miners) and 2MB blocks
./blockchain-sim -duration=2h -nodes=50 -miners=12 -block_size_bytes=2097152

# Run with faster block times (1-3 minutes) and lower tx rate
./blockchain-sim -find_time_min=1m -find_time_max=3m -tx_rate=5.0

# Run until 100k transactions are included (or 1h, whichever first)
./blockchain-sim -total_txs=100000 -duration=1h
