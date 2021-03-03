NAME = reweb
VERSION = latest
REPO = public.ecr.aws/apparentorder/reweb
TAG = $(shell date +%s)

all: build deploy

build:
	go build -o reweb -tags netgo -ldflags "-s -X main.Version=$(TAG)"

deploy:
	docker build -t $(NAME) .
	docker tag $(NAME) $(REPO):$(TAG)
	docker tag $(NAME) $(REPO):$(VERSION)
	docker push $(REPO):$(TAG)
	docker push $(REPO):$(VERSION)

