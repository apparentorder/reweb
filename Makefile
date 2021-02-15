NAME = reweb
REPO = public.ecr.aws/g2o8x4n0/reweb

all: build deploy

build:
	go build -o reweb -tags netgo -ldflags -s

TAG = $(shell date +%s)
deploy:
	docker build -t $(NAME) .
	docker tag $(NAME) $(REPO):$(TAG)
	docker tag $(NAME) $(REPO):latest
	docker push $(REPO):$(TAG)
	docker push $(REPO):latest

