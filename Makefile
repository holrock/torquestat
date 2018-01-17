.PHONY: all clean

EXE := torquestat
SRC := main.go bindata.go
ASSETS := $(wildcard template/*.html css/*.css)

all: $(EXE)

bindata.go: $(ASSETS)
	go-assets-builder -o bindata.go css template

$(EXE): $(SRC)
	go build -o $(EXE) $(SRC)

clean:
	$(RM) $(EXE) bindata.go

get:
	go get -v github.com/jessevdk/go-assets-builder

build_image:
	docker build -t torquestat .

docker:
	docker run --rm -v "$(PWD)":/b -w /b torquestat
