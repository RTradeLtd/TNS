# Set up test environment
.PHONY: testenv
WAIT=3
testenv:
	@echo "===================   preparing test env    ==================="
	( cd testenv ; make testenv )
	@echo "===================          done           ==================="

# Shut down testenv
.PHONY: stop-testenv
stop-testenv:
	@echo "===================  shutting down test env ==================="
	( cd testenv ; make stop-testenv )
	@echo "===================          done           ==================="

# Remove assets
.PHONY: clean
clean: stop-testenv
	@echo "=================== cleaning up temp assets ==================="
	@echo "Removing binary..."
	( cd testenv ; make clean )
	@echo "===================          done           ==================="

# go vet ./...
.PHONY: vet
vet:
	go vet ./...

# build proto buffs
.PHONY: proto
proto:
	protoc -I tns tns/pb/tns.proto --go_out=tns