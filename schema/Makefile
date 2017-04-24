
.PHONY: default
default: validate

.PHONY: help
help:
	@echo "Usage: make [target]"
	@echo
	@echo " * 'fmt' - format the json with indentation"
	@echo " * 'help' - show this help information"
	@echo " * 'validate' - build the validation tool"

.PHONY: fmt
fmt:
	for i in *.json ; do jq --indent 4 -M . "$${i}" > xx && cat xx > "$${i}" && rm xx ; done

.PHONY: validate
validate: validate.go
	go get -d ./...
	go build ./validate.go

.PHONY: clean
clean:
	rm -f validate
