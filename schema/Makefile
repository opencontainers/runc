GOOD_TESTS = $(wildcard test/good/*.json)
BAD_TESTS = $(wildcard test/bad/*.json)

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
	find . -name '*.json' -exec bash -c 'jq --indent 4 -M . {} > xx && mv xx {} || echo "skipping invalid {}"' \;

.PHONY: validate
validate: validate.go
	go get -d ./...
	go build ./validate.go

.PHONY: test
test: validate $(TESTS)
	for TYPE in $$(ls test); \
	do \
		echo "testing $${TYPE}"; \
		for FILE in $$(ls "test/$${TYPE}/good"); \
		do \
			echo "  testing test/$${TYPE}/good/$${FILE}"; \
			if ./validate "$${TYPE}-schema.json" "test/$${TYPE}/good/$${FILE}" ; \
			then \
				echo "    received expected validation success" ; \
			else \
				echo "    received unexpected validation failure" ; \
				exit 1; \
			fi \
		done; \
		for FILE in $$(ls "test/$${TYPE}/bad"); \
		do \
			echo "  testing test/$${TYPE}/bad/$${FILE}"; \
			if ./validate "$${TYPE}-schema.json" "test/$${TYPE}/good/$${FILE}" ; \
			then \
				echo "    received unexpected validation success" ; \
				exit 1; \
			else \
				echo "    received expected validation failure" ; \
			fi \
		done; \
	done

.PHONY: clean
clean:
	rm -f validate
