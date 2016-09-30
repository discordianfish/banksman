NAME     := $(shell basename $(CURDIR))
IPATH    := github.com/discordianfish/$(NAME)

REVISION := $(shell git rev-parse --short HEAD)
BRANCH   := $(shell git symbolic-ref --short -q HEAD)
DATE     := $(shell date +%Y%m%d-%H:%M:%S)
GOOS     ?= $(shell uname | tr A-Z a-z)
GOARCH   ?= $(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m)))

ifneq ($(CIRCLE_TAG),)
TAG := $(CIRCLE_TAG)
else
TAG := $(shell git describe --tags --exact-match || echo v0.0.0-dev)
endif

FNAME := $(NAME)-$(TAG).$(GOOS)-$(GOARCH)

ifneq ($(CIRCLE_ARTIFACTS),)
TARGET := $(CIRCLE_ARTIFACTS)
else
TARGET := .
endif

build: $(NAME)

$(NAME):
	go build -ldflags "\
		-X $(IPATH)/version.Version=${TAG} \
		-X $(IPATH)/version.Revision=${REVISION} \
		-X $(IPATH)/version.Branch=${BRANCH} \
		-X $(IPATH)/version.BuildUser=${USER} \
		-X $(IPATH)/version.BuildDate=${DATE} \
	" .
$(TARGET)/$(FNAME).tar.gz: $(NAME)
	tar czf $@ $<

$(TARGET)/$(FNAME).deb: $(NAME)
	fpm -v $(TAG:v%=%) \
		--url $(IPATH) -p $@ -n $< \
		--provides $(NAME) -t deb -s dir $<=/usr/bin/$(NAME)

release: $(TARGET)/$(FNAME).tar.gz $(TARGET)/$(FNAME).deb

clean:
	rm -rf $(TARGET)/$(FNAME).tar.gz $(TARGET)/$(FNAME).deb

.PHONY: build clean
