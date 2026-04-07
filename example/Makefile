SUBDIRS := hello-world sup-world

.PHONY: all install build test lint tidy clean

# Tip: running plain "make" from here will emulate all CI tasks!
all: tidy lint test

install:
	@baker -target install -title "I N S T A L L" -color white $(SUBDIRS)

build:
	@baker -target build -title "B U I L D" -color blue $(SUBDIRS)

test:
	@baker -target test -title "T E S T" -color green $(SUBDIRS)

lint:
	@baker -target lint -title "L I N T" -color yellow $(SUBDIRS)

tidy:
	@baker -target tidy -title "T I D Y" -color magenta $(SUBDIRS)

clean:
	@baker -target clean -title "C L E A N" -color cyan $(SUBDIRS)
