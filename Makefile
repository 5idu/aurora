.PHONY: 

up:
	@docker-compose -f scripts/etcd-docker-compose.yml up -d

down:
	@docker-compose -f scripts/etcd-docker-compose.yml down

protoc:
	@protoc -I=. -I/usr/local/include  --gogofast_out=. ./proto/api.proto