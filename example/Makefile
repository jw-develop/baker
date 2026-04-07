SUBDIRS := hello-world sup-world

.PHONY: build test lint clean

build:
	@baker -target build -title "B U I L D" -color blue $(SUBDIRS)

test:
	@baker -target test -title "T E S T" -color green $(SUBDIRS)

lint:
	@baker -target lint -title "L I N T" -color yellow $(SUBDIRS)

clean:
	@baker -target clean -title "C L E A N" -color magenta $(SUBDIRS)
