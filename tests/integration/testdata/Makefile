TEST_IMAGES := debian busybox hello-world
CONTAINER_ENGINE ?= docker

download_images:
	$(foreach image, $(TEST_IMAGES),$(shell ./download_oci_image.sh "$(image)"))
clean:
	-rm *.tar

.PHONY: download_images clean
