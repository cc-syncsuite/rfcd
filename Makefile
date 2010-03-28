include $(GOROOT)/src/Make.$(GOARCH)

TARG=rfcd
GOFILES=\
	main.go\
	clientcommands.go\
	rfcdconfig.go\

include $(GOROOT)/src/Make.cmd


