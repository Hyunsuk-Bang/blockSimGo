GO = go
GOFILES = *.go
EXECUTABLE = blockchain-sim

DURATION = 3h
TX_TOTAL = 50000 # Adjust as needed for duration/rate
#TX_RATE = 30
TX_RATE = 10
TX_SIZE_MIN = 150
TX_SIZE_MAX = 800
CONFIRM_DEPTH = 6
NETWORK_DELAY_MIN = 100ms
NETWORK_DELAY_MAX = 500ms

BASELINE_NODES = 20
BASELINE_MINERS = 5
BASELINE_BLOCK_SIZE = 1048576 # 1 MiB
BASELINE_FIND_TIME_MIN = 9m
BASELINE_FIND_TIME_MAX = 11m

NODE_VALUES = 20 50 100
NODE_MINERS = 10 25 50
BLOCKSIZE_VALUES = 524288 1048576 2097152 4194304
FIND_TIME_PAIRS = 1m_3m 5m_7m 9m_11m 13m_15m 17m_19m
FIND_TIME_MIN_VALS = 2m 4m 10m 16m
FIND_TIME_MAX_VALS = 3m 7m 11m 17m

BASELINE_FLAGS = -duration=$(DURATION) -total_txs=$(TX_TOTAL) -tx_rate=$(TX_RATE) \
                 -tx_size_min=$(TX_SIZE_MIN) -tx_size_max=$(TX_SIZE_MAX) \
                 -confirm_depth=$(CONFIRM_DEPTH) \
                 -delay_min=$(NETWORK_DELAY_MIN) -delay_max=$(NETWORK_DELAY_MAX)

.PHONY: all build test-all test_nodes test_blocksize test_interval clean run_baseline \
        run_node_N20_M5 run_node_N50_M12 run_node_N100_M25 \
        $(foreach b, $(BLOCKSIZE_VALUES), run_bsize_$(b)) \
        run_interval_5m_7m run_interval_9m_11m run_interval_14m_16m

all: build

build: $(EXECUTABLE)

$(EXECUTABLE): $(GOFILES)
	@echo "Building $(EXECUTABLE)..."
	$(GO) build -o $(EXECUTABLE) $(GOFILES)
	@echo "Build complete."

test-all: test_nodes test_blocksize test_interval
	@echo "All test runs initiated. Check individual log files."

# 1. Baseline Run (explicitly)
# Note: This configuration might be run again within other test groups.
run_baseline: $(EXECUTABLE)
	@echo "--- Running Baseline Test (N=$(BASELINE_NODES), M=$(BASELINE_MINERS), B=$(BASELINE_BLOCK_SIZE), I=$(BASELINE_FIND_TIME_MIN)-$(BASELINE_FIND_TIME_MAX)) ---"
	./$(EXECUTABLE) $(BASELINE_FLAGS) \
		-nodes=$(BASELINE_NODES) -miners=$(BASELINE_MINERS) \
		-block_size_bytes=$(BASELINE_BLOCK_SIZE) \
		-find_time_min=$(BASELINE_FIND_TIME_MIN) -find_time_max=$(BASELINE_FIND_TIME_MAX) \
		> run_log_baseline_N$(BASELINE_NODES)_M$(BASELINE_MINERS)_B$(BASELINE_BLOCK_SIZE)_I$(BASELINE_FIND_TIME_MIN)-$(BASELINE_FIND_TIME_MAX).txt
	@echo "Baseline Test Done. Log: run_log_baseline..."


# 2. Test Varying Network Size
# Rule template to generate run targets for each node/miner pair
define RunNodesTemplate
run_node_N$(1)_M$(2): $(EXECUTABLE)
	@echo "--- Running Network Size Test (Nodes=$(1), Miners=$(2)) ---"
	./$(EXECUTABLE) $(BASELINE_FLAGS) \
		-nodes=$(1) -miners=$(2) \
		-block_size_bytes=$(BASELINE_BLOCK_SIZE) \
		-find_time_min=$(BASELINE_FIND_TIME_MIN) -find_time_max=$(BASELINE_FIND_TIME_MAX) \
		| tee run_log_nodes_N$(1)_M$(2).txt
	@echo "Network Size Test (N=$(1), M=$(2)) Done. Log: run_log_nodes_N$(1)_M$(2).txt"
endef
$(eval $(call RunNodesTemplate,20,10))
$(eval $(call RunNodesTemplate,50,25))
$(eval $(call RunNodesTemplate,100,50))
test_nodes: run_node_N20_M10 run_node_N50_M25 run_node_N100_M50


# 3. Test Varying Block Size
# Rule template to generate run targets for each block size value
define RunBlockSizeTemplate
run_bsize_$(1): $(EXECUTABLE)
	@echo "--- Running Block Size Test (BlockSize=$(1)) ---"
	./$(EXECUTABLE) $(BASELINE_FLAGS) \
		-nodes=$(BASELINE_NODES) -miners=$(BASELINE_MINERS) \
		-block_size_bytes=$(1) \
		-find_time_min=$(BASELINE_FIND_TIME_MIN) -find_time_max=$(BASELINE_FIND_TIME_MAX) \
		> run_log_blocksize_B$(1).txt
	@echo "Block Size Test (B=$(1)) Done. Log: run_log_blocksize_B$(1).txt"
endef
# Instantiate the template for each value
$(foreach b, $(BLOCKSIZE_VALUES), $(eval $(call RunBlockSizeTemplate,$(b))))
# Target that depends on all individual block size runs
test_blocksize: $(foreach b, $(BLOCKSIZE_VALUES), run_bsize_$(b))


# 4. Test Varying Find Time Interval Range
# Rule template to generate run targets for each min/max pair
define RunIntervalTemplate
run_interval_$(1)_$(2): $(EXECUTABLE)
	@echo "--- Running Interval Range Test (Min=$(1), Max=$(2)) ---"
	./$(EXECUTABLE) $(BASELINE_FLAGS) \
		-nodes=$(BASELINE_NODES) -miners=$(BASELINE_MINERS) \
		-block_size_bytes=$(BASELINE_BLOCK_SIZE) \
		-find_time_min=$(1) -find_time_max=$(2) \
		> run_log_interval_I$(1)-$(2).txt
	@echo "Interval Range Test (Min=$(1), Max=$(2)) Done. Log: run_log_interval_I$(1)-$(2).txt"
endef
# Instantiate the template for each pair (manual pairing based on lists)
$(eval $(call RunIntervalTemplate,1m,3m))
$(eval $(call RunIntervalTemplate,5m,7m))
$(eval $(call RunIntervalTemplate,9m,11m))
$(eval $(call RunIntervalTemplate,13m,15m))
$(eval $(call RunIntervalTemplate,17m,19m))
$(eval $(call RunIntervalTemplate,21m,23m))
# Target that depends on all individual interval range runs
test_interval: run_interval_1m_3m run_interval_5m_7m run_interval_9m_11m run_interval_13m_15m run_interval_17m_19m run_interval_21m_23m


# Clean up build artifacts and logs
clean:
	@echo "Cleaning up..."
	$(GO) clean
	rm -f $(EXECUTABLE) run_log_*.txt
	@echo "Cleanup complete."
