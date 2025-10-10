#!/bin/bash
set -e
source .env

start_local_container() {
  if [ -n "$CONTAINER_ID" ]; then
    echo "Using existing container: $CONTAINER_ID"
  else
		CONTAINER_ID=$(docker run \
      --name $CONTAINER_NAME \
  		-v /var/run/docker.sock:/var/run/docker.sock \
			-v "$(pwd):/app/op-geth/test-pp-op" \
  		-v "$(pwd)/data/op-geth-seq:/app/op-geth/test-pp-op/data/op-geth-seq" \
  		-v "$(pwd)/data/cannon-data:/app/op-program/bin" \
  		-e DOCKER_HOST=unix:///var/run/docker.sock \
  		-d op-migrate:latest sleep infinity)
  fi
}

start_testnet_container() {
  if [ -n "$CONTAINER_ID" ]; then
    echo "Using existing container: $CONTAINER_ID"
  else
    CONTAINER_ID=$(docker run \
      --name $CONTAINER_NAME \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -v /data/erigon-data:/data/erigon-data \
      -v /mnt/ramdisk_op:/mnt/ramdisk_op \
      -v /mnt/ramdisk_op/test-pp-op/data/op-geth-seq:/app/op-geth/test-pp-op/data/op-geth-seq \
      -v /mnt/ramdisk_op/test-pp-op/data/cannon-data:/app/op-program/bin \
      -e DOCKER_HOST=unix:///var/run/docker.sock \
      -d op-migrate:amd64 sleep infinity)
  fi
}

# Default values
CONTAINER_NAME="op-migrate-container"
CONTAINER_ID=$(docker ps -aq -f name=^/${CONTAINER_NAME}$)
STAGE=1

while [[ $# -gt 0 ]]; do
  case $1 in
    -s|--stage)
      STAGE=$2
			if (( STAGE < 1 || STAGE > 3 ));then
				echo "STAGE must lie between 1 and 3 (inclusive)."
				exit 1
			fi
      shift 2
      ;;
    --ssh)
			echo "ssh"

			if [ "$ENV" = "local" ];then
				echo "ssh local"
				start_local_container
				docker exec -it $CONTAINER_ID /bin/bash
			elif [ "$ENV" = "testnet" ];then
				echo "ssh testnet"
				start_testnet_container
				docker exec -it $CONTAINER_ID /bin/bash
			else
				echo "Only ENV values (local, testnet) are accepted."
				exit 1
			fi
			exit 0
      ;;
    -h|--help)
      echo "Usage: $0 [OPTIONS]"
      echo "Options:"
      echo "  --stage       Select stage to start from (1,2, or 3)."
      echo "  --ssh         SSH into existing container. Otherwise create new container and SSH into it." 
      echo "  -h, --help    Show this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Use --help for usage information"
      exit 1
      ;;
  esac
done

prompt_before_continue() {
	read -p "Do you want to continue? (y/n): " answer
	case $answer in
  	[Yy]* ) echo "Continuing...";;
  	[Nn]* ) echo "Exiting..."; exit;;
  	* ) echo "Invalid input";;
	esac
}

echo "STAGE = $STAGE"
if [ "$ENV" = "local" ];then

	start_local_container

	if (( STAGE <= 1 ));then
  	docker exec $CONTAINER_ID bash -c "
  		cd /app/op-geth/test-pp-op
  		./5-1-migrate-prepare.sh
		"
		prompt_before_continue
	fi

	if (( STAGE <= 2 ));then
  	docker exec $CONTAINER_ID bash -c "
  		cd /app/op-geth/test-pp-op
  		./5-2-migrate-op.sh
			echo 'Compressing merged.genesis.json...'
  		gzip -c merged.genesis.json > config-op/merged.genesis.gz.json
  		cp config-op/rollup.json /app/op-program/chainconfig/configs/195-rollup.json
  		cp config-op/merged.genesis.gz.json /app/op-program/chainconfig/configs/195-genesis-l2.json
		"
		prompt_before_continue
	fi

	if (( STAGE <= 3 ));then
  	docker exec $CONTAINER_ID bash -c "
  		cd /app
			make reproducible-prestate
		"
	fi
fi

if [ "$ENV" = "testnet" ];then

	start_testnet_container

	if (( STAGE <= 1 ));then
  	docker exec $CONTAINER_ID bash -c "
  		cd /app/op-geth/test-pp-op
  		./5-1-migrate-prepare.sh
		"
		prompt_before_continue
	fi

	if (( STAGE <= 2 ));then
  	docker exec $CONTAINER_ID bash -c "
  		cd /app/op-geth/test-pp-op
  		./5-2-migrate-op.sh
			gzip -c merged.genesis.json > config-op/merged.genesis.gz.json                              
			echo 'Compressing merged.genesis.json...'
			cp config-op/rollup.json /app/op-program/chainconfig/configs/196-rollup.json                
			cp config-op/merged.genesis.gz.json /app/op-program/chainconfig/configs/196-genesis-l2.json 
			cp dockerfile/Dockerfile.repro /app/op-program/Dockerfile.repro                             
		"
		prompt_before_continue
	fi

	if (( STAGE <= 3 ));then
  	docker exec $CONTAINER_ID bash -c "
  		cd /app
			make reproducible-prestate
	    cp -rfv /app/op-geth/test-pp-op/* /app/op-geth/test-pp-op/.* /mnt/ramdisk_op/test-pp-op
		"
	fi
fi
