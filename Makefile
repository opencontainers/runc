
all:
	docker build -t docker/libcontainer .

test:
	# we need NET_ADMIN for the netlink tests
	docker run --rm --cap-add NET_ADMIN docker/libcontainer
