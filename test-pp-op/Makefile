ifeq ($(ENV),local)
    COMPOSE_FILE := docker-compose-local.yml
	include local.env
else
    COMPOSE_FILE := docker-compose.yml
	include testnet.env
endif

DOCKER_COMPOSE := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose") -f ${COMPOSE_FILE}

RUN_DOCKER_MOCK_L1_NETWORK := $(DOCKER_COMPOSE) up -d xlayer-mock-l1-network

PAUSE := $(DOCKER_COMPOSE) down --remove-orphans; \
	sleep 3; \
	rm -Rf ./l1-geth/consensus/beacondata; \
	rm -Rf ./l1-geth/consensus/validatordata; \
	rm -Rf ./l1-geth/consensus/genesis.ssz; \
	rm -Rf ./l1-geth/execution/geth; \
    rm -Rf ./config-op/artifact.json; \
	rm -Rf ./config-op/31337-deploy.json; \
    rm -Rf ./config-op/genesis.json; \
	rm -Rf ./config-op/rollup.json; \
	rm -Rf ./config-op/state_dump.json; \
	rm -Rf ./l1-geth/execution/genesis.json; \
	rm -Rf ./config-op/genesis-op-raw.json; \
	rm -Rf ./config-op/superchain.json; \
	rm -Rf ./config-op/implementations.json; \
	git checkout config-op/intent.toml; \
	git checkout config-op/state.json; \


DEPLOY_ERC20_L1 := cd contracts && forge create OKBToken.sol:StandardERC20 --private-key $(SEQUENCER_PRIVATE_KEY) --rpc-url $(L1_RPC_URL) --legacy --broadcast --constructor-args "OKBToken" "OKB" 1000000000
INIT_ACCOUNT_1 := cast send -f $(SEQUENCER_ADDRESS)  --private-key $(SEQUENCER_PRIVATE_KEY) --value 1000ether $(DEPLOYER_ADDRESS) --legacy --rpc-url $(L1_RPC_URL)
INIT_ACCOUNT_2 := cast send -f $(SEQUENCER_ADDRESS)  --private-key $(SEQUENCER_PRIVATE_KEY) --value 1000ether $(AGGREGATOR_ADDRESS) --legacy --rpc-url $(L1_RPC_URL)
INIT_ACCOUNT_3 := cast send -f $(SEQUENCER_ADDRESS)  --private-key $(SEQUENCER_PRIVATE_KEY) --value 1000ether $(RICH_ADDRESS) --legacy --rpc-url $(L1_RPC_URL)
INIT_ACCOUNT_4 := cast send -f $(SEQUENCER_ADDRESS)  --private-key $(SEQUENCER_PRIVATE_KEY) --value 1000ether $(CHALLENGER_ADDRESS) --legacy --rpc-url $(L1_RPC_URL)

INIT_BRIDGE_ACCOUNT := cast send -f $(DEPLOYER_ADDRESS)  --private-key $(DEPLOYER_PRIVATE_KEY) --value 0.01ether $(SEQUENCER_ADDRESS) --legacy --rpc-url $(L2_RPC_URL)

.PHONY: run
run: stop build-docker
	@echo "🚀 Starting L1 PoS environment automatic dependency management..."
	$(DOCKER_COMPOSE) up -d l1-validator
	@echo "🎉 L1 PoS environment started!"
	sleep 30
	$(DEPLOY_ERC20_L1)
	$(INIT_ACCOUNT_1)
	$(INIT_ACCOUNT_2)
	$(INIT_ACCOUNT_3)
	$(INIT_ACCOUNT_4)
	sleep 5
	./init.sh
	sleep 5
	$(DOCKER_COMPOSE) up -d xlayer-bridge-db
	$(DOCKER_COMPOSE) up -d xlayer-pool-db
	sleep 5
	$(DOCKER_COMPOSE) up -d xlayer-agglayer-prover
	$(DOCKER_COMPOSE) up -d xlayer-agglayer
	sleep 5

	sleep 3
	$(DOCKER_COMPOSE) up -d xlayer-approve
	$(DOCKER_COMPOSE) up -d xlayer-seq
	$(DOCKER_COMPOSE) up -d xlayer-pool-manager

	sleep 5
	$(DOCKER_COMPOSE) up -d xlayer-rpc

	sleep 10
	$(DOCKER_COMPOSE) up -d xlayer-bridge-service
	$(DOCKER_COMPOSE) up -d xlayer-bridge-ui
	$(INIT_BRIDGE_ACCOUNT)

	$(DOCKER_COMPOSE) up -d xlayer-agg-sender

.PHONY: mainnet
mainnet:
	@echo "🚀 Starting XLayer Fake Mainnet ..."
	$(DOCKER_COMPOSE) up -d l1-validator
	sleep 20
	$(INIT_ACCOUNT_1)
	$(INIT_ACCOUNT_2)
	$(INIT_ACCOUNT_3)
	$(INIT_ACCOUNT_4)
	$(DOCKER_COMPOSE) up -d xlayer-mainnet-seq
	sleep 5
	$(DOCKER_COMPOSE) up -d xlayer-rpc


.PHONY: set-env
set-env:
	@if [ "$(ENV)" = "local" ]; then \
		if [ -f local.env ]; then \
			cp local.env .env; \
			echo "Using local.env -> .env"; \
		else \
			echo "local.env not found"; \
			exit 1; \
		fi; \
	fi

.PHONY: show-env
show-env:
	@echo "Current ENV: $(ENV)"
	@echo "Using compose file: $(COMPOSE_FILE)"

.PHONY: run_erigon
run_erigon: stop set-env show-env ## Runs a minimal node
	echo $(DOCKER_RPC)
	$(RUN_DOCKER_MOCK_L1_NETWORK)
	sleep 3
	$(DOCKER_COMPOSE) up -d xlayer-seq
	$(RUN_DOCKER_PROMETHEUS)
	$(RUN_DOCKER_GRAFANA)
	sleep 5
	$(DOCKER_COMPOSE) up -d xlayer-rpc


.PHONY: pause
pause: ## Stops all services
	$(PAUSE)

.PHONY: stop
stop: pause
	rm -rf data


.PHONY: build-docker
build-docker: ## Build cdk-erigoin for sequencer and rpc
	#docker build -t cdk-erigon -f ../Dockerfile.local ../
	./setup_images.sh

.PHONY: clean
clean: stop
	rm -rf data
	rm -rf data_state*
	rm -rf tmp
	rm -f config-op/genesis*.json
	rm -f config-op/state*.json
	rm -f sa-tx-after.txt sa-tx-before.txt tx-fee-after.txt tx-fee-before.txt